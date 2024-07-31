import "dotenv/config";
import { formatEther, formatUnits } from "viem";

export const METAMASK_SEED_PHRASE = process.env.E2E_TEST_SEED_PHRASE;
export const METAMASK_PASSWORD = process.env.E2E_TEST_WALLET_PASSWORD;
export const TEST_PRIVATE_KEY = process.env.E2E_TEST_PRIVATE_KEY;
export const INFURA_PROJECT_ID = process.env.NEXT_PUBLIC_INFURA_ID;

export const LINEA_SEPOLIA_NETWORK = {
  name: "Linea Sepolia",
  rpcUrl: `https://linea-sepolia.infura.io/v3/${INFURA_PROJECT_ID}`,
  chainId: 59141,
  symbol: "LineaETH",
  blockExplorerUrl: "https://sepolia.lineascan.build",
};

export const TEST_URL = "http://localhost:3000/";
export const SEPOLIA_NETWORK_NAME = "Sepolia";
export const WEI_AMOUNT = formatEther(BigInt(1)).toString();
export const USDC_AMOUNT = formatUnits(BigInt(1), 6).toString();
