package net.consensys.zkevm.ethereum.coordination.blob

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.vertx.core.Handler
import io.vertx.core.Vertx
import kotlinx.datetime.Instant
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.LongRunningService
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClient
import net.consensys.zkevm.domain.Blob
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.BlobStatus
import net.consensys.zkevm.domain.BlockInterval
import net.consensys.zkevm.domain.BlockIntervals
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.toBlockIntervalsString
import net.consensys.zkevm.ethereum.coordination.conflation.BlobCreationHandler
import net.consensys.zkevm.persistence.blob.BlobsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ArrayBlockingQueue
import java.util.concurrent.CompletableFuture
import kotlin.time.Duration

class BlobCompressionProofCoordinator(
  private val vertx: Vertx,
  private val blobsRepository: BlobsRepository,
  private val blobCompressionProverClient: BlobCompressionProverClient,
  private val rollingBlobShnarfCalculator: RollingBlobShnarfCalculator,
  private val blobZkStateProvider: BlobZkStateProvider,
  private val config: Config,
  private val blobCompressionProofHandler: BlobCompressionProofHandler,
  metricsFacade: MetricsFacade
) : BlobCreationHandler, LongRunningService {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val defaultQueueCapacity = 1000 // Should be more than blob submission limit
  private val blobsToHandle = ArrayBlockingQueue<Blob>(defaultQueueCapacity)
  private var timerId: Long? = null
  private lateinit var blobPollingAction: Handler<Long>
  private val blobsCounter = metricsFacade.createCounter(
    LineaMetricsCategory.BLOB,
    "counter",
    "New blobs arriving to blob compression proof coordinator"
  )

  init {
    metricsFacade.createGauge(
      LineaMetricsCategory.BLOB,
      "compression.queue.size",
      "Size of blob compression proving queue",
      { blobsToHandle.size }
    )
  }

  data class Config(
    val pollingInterval: Duration
  )

  @Synchronized
  private fun sendBlobToCompressionProver(blob: Blob): SafeFuture<Unit> {
    log.debug(
      "Going to create the blob compression proof for ${blob.intervalString()}"
    )
    val blobZkSateAndRollingShnarfFuture = blobZkStateProvider.getBlobZKState(blob.blocksRange)
      .thenCompose { blobZkState ->
        rollingBlobShnarfCalculator.calculateShnarf(
          compressedData = blob.compressedData,
          parentStateRootHash = blobZkState.parentStateRootHash,
          finalStateRootHash = blobZkState.finalStateRootHash,
          conflationOrder = BlockIntervals(
            startingBlockNumber = blob.conflations.first().startBlockNumber,
            upperBoundaries = blob.conflations.map { it.endBlockNumber }
          )
        ).thenApply { rollingBlobShnarfResult ->
          Pair(blobZkState, rollingBlobShnarfResult)
        }
      }

    blobZkSateAndRollingShnarfFuture.thenCompose { (blobZkState, rollingBlobShnarfResult) ->
      requestBlobCompressionProof(
        compressedData = blob.compressedData,
        conflations = blob.conflations,
        parentStateRootHash = blobZkState.parentStateRootHash,
        finalStateRootHash = blobZkState.finalStateRootHash,
        parentDataHash = rollingBlobShnarfResult.parentBlobHash,
        prevShnarf = rollingBlobShnarfResult.parentBlobShnarf,
        expectedShnarfResult = rollingBlobShnarfResult.shnarfResult,
        commitment = rollingBlobShnarfResult.shnarfResult.commitment,
        kzgProofContract = rollingBlobShnarfResult.shnarfResult.kzgProofContract,
        kzgProofSideCar = rollingBlobShnarfResult.shnarfResult.kzgProofSideCar,
        blobStartBlockTime = blob.startBlockTime,
        blobEndBlockTime = blob.endBlockTime
      ).whenException { exception ->
        log.error(
          "Error in requesting blob compression proof: blob={} errorMessage={} ",
          blob.intervalString(),
          exception.message,
          exception
        )
      }
    }
    // We want to process the next blob without waiting for the compression proof to finish and process the next
    // blob after shnarf calculation of current blob is done
    return blobZkSateAndRollingShnarfFuture.thenApply {}
  }

  private fun requestBlobCompressionProof(
    compressedData: ByteArray,
    conflations: List<ConflationCalculationResult>,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    parentDataHash: ByteArray,
    prevShnarf: ByteArray,
    expectedShnarfResult: ShnarfResult,
    commitment: ByteArray,
    kzgProofContract: ByteArray,
    kzgProofSideCar: ByteArray,
    blobStartBlockTime: Instant,
    blobEndBlockTime: Instant
  ): SafeFuture<Unit> {
    return blobCompressionProverClient.requestBlobCompressionProof(
      compressedData = compressedData,
      conflations = conflations,
      parentStateRootHash = parentStateRootHash,
      finalStateRootHash = finalStateRootHash,
      parentDataHash = parentDataHash,
      prevShnarf = prevShnarf,
      expectedShnarfResult = expectedShnarfResult,
      commitment = commitment,
      kzgProofContract = kzgProofContract,
      kzgProofSideCar = kzgProofSideCar
    ).thenCompose { result ->
      if (result is Err) {
        SafeFuture.failedFuture(result.error.asException())
      } else {
        val blobCompressionProof = (result as Ok).value
        val blobRecord = BlobRecord(
          startBlockNumber = conflations.first().startBlockNumber,
          endBlockNumber = conflations.last().endBlockNumber,
          blobHash = expectedShnarfResult.dataHash,
          startBlockTime = blobStartBlockTime,
          endBlockTime = blobEndBlockTime,
          batchesCount = conflations.size.toUInt(),
          status = BlobStatus.COMPRESSION_PROVEN,
          expectedShnarf = expectedShnarfResult.expectedShnarf,
          blobCompressionProof = blobCompressionProof
        )
        SafeFuture.allOf(
          blobsRepository.saveNewBlob(blobRecord),
          blobCompressionProofHandler.acceptNewBlobCompressionProof(
            BlobCompressionProofUpdate(
              blockInterval = BlockInterval.between(
                startBlockNumber = blobRecord.startBlockNumber,
                endBlockNumber = blobRecord.endBlockNumber
              ),
              blobCompressionProof = blobCompressionProof
            )
          )
        ).thenApply {}
      }
    }
  }

  @Synchronized
  override fun handleBlob(blob: Blob): SafeFuture<*> {
    blobsCounter.increment()
    log.debug(
      "new blob: blob={} queuedBlobsToProve={} blobBatches={}",
      blob.intervalString(),
      blobsToHandle.size,
      blob.conflations.toBlockIntervalsString()
    )
    blobsToHandle.put(blob)
    log.trace("Blob was added to the handling queue {}", blob)
    return SafeFuture.completedFuture(Unit)
  }

  override fun start(): CompletableFuture<Unit> {
    if (timerId == null) {
      blobPollingAction = Handler<Long> {
        handleBlobsFromTheQueue().whenComplete { _, error ->
          error?.let {
            log.error("Error polling blobs for aggregation: errorMessage={}", error.message, error)
          }
          timerId = vertx.setTimer(config.pollingInterval.inWholeMilliseconds, blobPollingAction)
        }
      }
      timerId = vertx.setTimer(config.pollingInterval.inWholeMilliseconds, blobPollingAction)
    }
    return SafeFuture.completedFuture(Unit)
  }

  private fun handleBlobsFromTheQueue(): SafeFuture<Unit> {
    var blobsHandlingFuture = SafeFuture.completedFuture(Unit)
    if (blobsToHandle.isNotEmpty()) {
      val blobToHandle = blobsToHandle.poll()
      blobsHandlingFuture = blobsHandlingFuture.thenCompose {
        sendBlobToCompressionProver(blobToHandle).whenException { exception ->
          log.error(
            "Error in sending blob to compression prover: blob={} errorMessage={} ",
            blobToHandle.intervalString(),
            exception.message,
            exception
          )
        }
      }
    }
    return blobsHandlingFuture
  }

  override fun stop(): CompletableFuture<Unit> {
    if (timerId != null) {
      vertx.cancelTimer(timerId!!)
      blobPollingAction = Handler<Long> {}
    }
    return SafeFuture.completedFuture(Unit)
  }
}
