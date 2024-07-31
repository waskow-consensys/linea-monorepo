package net.consensys.zkevm.coordinator.app

import com.fasterxml.jackson.databind.module.SimpleModule
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.core.json.jackson.DatabindCodec
import io.vertx.micrometer.backends.BackendRegistries
import io.vertx.sqlclient.SqlClient
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.contract.Web3JL2MessageServiceLogsClient
import net.consensys.linea.contract.Web3JLogsClient
import net.consensys.linea.jsonrpc.client.LoadBalancingJsonRpcClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.vertx.loadVertxConfig
import net.consensys.linea.web3j.okHttpClientBuilder
import net.consensys.zkevm.coordinator.api.Api
import net.consensys.zkevm.coordinator.clients.prover.FileBasedExecutionProverClient
import net.consensys.zkevm.persistence.dao.aggregation.AggregationsRepositoryImpl
import net.consensys.zkevm.persistence.dao.aggregation.PostgresAggregationsDao
import net.consensys.zkevm.persistence.dao.aggregation.RetryingPostgresAggregationsDao
import net.consensys.zkevm.persistence.dao.batch.persistence.BatchesPostgresDao
import net.consensys.zkevm.persistence.dao.batch.persistence.PostgresBatchesRepository
import net.consensys.zkevm.persistence.dao.batch.persistence.RetryingBatchesPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsPostgresDao
import net.consensys.zkevm.persistence.dao.blob.BlobsRepositoryImpl
import net.consensys.zkevm.persistence.dao.blob.RetryingBlobsPostgresDao
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesPostgresDao
import net.consensys.zkevm.persistence.dao.feehistory.FeeHistoriesRepositoryImpl
import net.consensys.zkevm.persistence.db.Db
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import org.web3j.utils.Async
import tech.pegasys.teku.ethereum.executionclient.serialization.BytesSerializer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.toKotlinDuration

class CoordinatorApp(private val configs: CoordinatorConfig) {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val vertx: Vertx = run {
    log.trace("System properties: {}", System.getProperties())
    val vertxConfig = loadVertxConfig()
    log.debug("Vertx full configs: {}", vertxConfig)
    log.info("App configs: {}", configs)

    // TODO: adapt JsonMessageProcessor to use custom ObjectMapper
    // this is just dark magic.
    val module = SimpleModule()
    module.addSerializer(Bytes::class.java, BytesSerializer())
    DatabindCodec.mapper().registerModule(module)
    // .enable(SerializationFeature.INDENT_OUTPUT)
    Vertx.vertx(vertxConfig)
  }
  private val meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow()
  private val httpJsonRpcClientFactory = VertxHttpJsonRpcClientFactory(
    vertx,
    meterRegistry,
    requestResponseLogLevel = Level.TRACE,
    failuresLogLevel = Level.WARN
  )
  private val api = Api(
    Api.Config(
      configs.api.observabilityPort
    ),
    vertx
  )
  private val l2Web3jClient: Web3j =
    Web3j.build(
      HttpService(
        configs.l2.rpcEndpoint.toString(),
        okHttpClientBuilder(LogManager.getLogger("clients.l2")).build()
      ),
      1000,
      Async.defaultExecutorService()
    )

  private fun createExecutionProverClient(config: ProverConfig): FileBasedExecutionProverClient {
    return FileBasedExecutionProverClient(
      config = FileBasedExecutionProverClient.Config(
        requestDirectory = config.fsRequestsDirectory,
        responseDirectory = config.fsResponsesDirectory,
        inprogressProvingSuffixPattern = config.fsInprogressProvingSuffixPattern,
        pollingInterval = config.fsPollingInterval.toKotlinDuration(),
        timeout = config.fsPollingTimeout.toKotlinDuration(),
        tracesVersion = configs.traces.rawExecutionTracesVersion,
        stateManagerVersion = configs.stateManager.version
      ),
      l2MessageServiceLogsClient = Web3JL2MessageServiceLogsClient(
        logsClient = Web3JLogsClient(vertx, l2Web3jClient),
        l2MessageServiceAddress = configs.l2.messageServiceAddress
      ),
      vertx = vertx,
      l2Web3jClient = l2Web3jClient
    )
  }

  private val proverClient: FileBasedExecutionProverClient = createExecutionProverClient(configs.prover)

  private val persistenceRetryer = PersistenceRetryer(
    vertx = vertx,
    config = PersistenceRetryer.Config(
      backoffDelay = configs.persistenceRetry.backoffDelay.toKotlinDuration(),
      maxRetries = configs.persistenceRetry.maxRetries,
      timeout = configs.persistenceRetry.timeout?.toKotlinDuration()
    )
  )

  private val sqlClient: SqlClient = initDb(configs.database)
  private val batchesRepository =
    PostgresBatchesRepository(
      batchesDao = RetryingBatchesPostgresDao(
        delegate = BatchesPostgresDao(
          connection = sqlClient
        ),
        persistenceRetryer = persistenceRetryer
      )
    )

  private val blobsRepository =
    BlobsRepositoryImpl(
      blobsDao = RetryingBlobsPostgresDao(
        delegate = BlobsPostgresDao(
          config = BlobsPostgresDao.Config(
            maxBlobsToReturn = configs.blobSubmission.maxBlobsToReturn.toUInt()
          ),
          connection = sqlClient
        ),
        persistenceRetryer = persistenceRetryer
      )
    )

  private val aggregationsRepository = AggregationsRepositoryImpl(
    aggregationsPostgresDao = RetryingPostgresAggregationsDao(
      delegate = PostgresAggregationsDao(
        connection = sqlClient
      ),
      persistenceRetryer = persistenceRetryer
    )
  )

  private val micrometerMetricsFacade = MicrometerMetricsFacade(meterRegistry, "linea")

  private val l1FeeHistoriesRepository =
    FeeHistoriesRepositoryImpl(
      FeeHistoriesRepositoryImpl.Config(
        rewardPercentiles = configs.l1DynamicGasPriceCapService.feeHistoryFetcher.rewardPercentiles,
        minBaseFeePerBlobGasToCache =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.historicBaseFeePerBlobGasLowerBound,
        fixedAverageRewardToCache =
        configs.l1DynamicGasPriceCapService.gasPriceCapCalculation.historicAvgRewardConstant
      ),
      FeeHistoriesPostgresDao(
        sqlClient
      )
    )

  private val l1App = L1DependentApp(
    configs = configs,
    vertx = vertx,
    l2Web3jClient = l2Web3jClient,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    proverClientV2 = proverClient,
    batchesRepository = batchesRepository,
    blobsRepository = blobsRepository,
    aggregationsRepository = aggregationsRepository,
    l1FeeHistoriesRepository = l1FeeHistoriesRepository,
    smartContractErrors = configs.conflation.smartContractErrors,
    metricsFacade = micrometerMetricsFacade
  )

  private val requestFileCleanup = DirectoryCleaner(
    vertx = vertx,
    directories = listOf(
      configs.prover.fsRequestsDirectory, // Execution proof request directory
      configs.blobCompression.prover.fsRequestsDirectory, // Compression proof request directory
      configs.proofAggregation.prover.fsRequestsDirectory // Aggregation proof request directory
    ),
    fileFilters = DirectoryCleaner.getSuffixFileFilters(
      listOf(
        configs.prover.fsInprogressRequestWritingSuffix,
        configs.blobCompression.prover.fsInprogressRequestWritingSuffix,
        configs.proofAggregation.prover.fsInprogressRequestWritingSuffix
      )
    ) + DirectoryCleaner.JSON_FILE_FILTER
  )

  init {
    log.info("Coordinator app instantiated")
  }

  fun start() {
    requestFileCleanup.cleanup()
      .thenCompose { l1App.start() }
      .thenCompose { api.start().toSafeFuture() }
      .get()

    log.info("Started :)")
  }

  fun stop(): Int {
    SafeFuture.allOf(
      l1App.stop(),
      SafeFuture.fromRunnable { l2Web3jClient.shutdown() },
      api.stop().toSafeFuture()
    ).thenApply {
      LoadBalancingJsonRpcClient.stop()
    }.thenCompose {
      requestFileCleanup.cleanup()
    }.thenCompose {
      vertx.close().toSafeFuture().thenApply { log.info("vertx Stopped") }
    }.thenApply {
      log.info("CoordinatorApp Stopped")
    }.get()
    return 0
  }

  private fun initDb(dbConfig: DatabaseConfig): SqlClient {
    val dbVersion = "4"
    Db.applyDbMigrations(
      host = dbConfig.host,
      port = dbConfig.port,
      database = dbConfig.schema,
      target = dbVersion,
      username = dbConfig.username,
      password = dbConfig.password.value
    )
    return Db.vertxSqlClient(
      vertx = vertx,
      host = dbConfig.host,
      port = dbConfig.port,
      database = dbConfig.schema,
      username = dbConfig.username,
      password = dbConfig.password.value,
      maxPoolSize = dbConfig.transactionalPoolSize,
      pipeliningLimit = dbConfig.readPipeliningLimit
    )
  }
}
