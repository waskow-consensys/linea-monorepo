"use client";

import { useEffect, useState } from "react";
import { useFormContext } from "react-hook-form";
import { useAccount, useWaitForTransactionReceipt } from "wagmi";
import { parseUnits } from "viem";
import classNames from "classnames";
import { toast } from "react-toastify";
import { useSwitchNetwork, useAllowance, useApprove } from "@/hooks";
import { Transaction } from "@/models";
import { useChainStore } from "@/stores/chainStore";

export type BridgeForm = {
  amount: string;
  balance: string;
  submit: string;
};

export default function Approve() {
  const [waitingTransaction, setWaitingTransaction] = useState<Transaction | undefined>();

  // Form
  const { getValues, setValue, watch } = useFormContext();
  const watchAmount = watch("amount", false);
  const watchBalance = watch("balance", false);

  // Context
  const { token, fromChain, tokenBridgeAddress } = useChainStore((state) => ({
    token: state.token,
    fromChain: state.fromChain,
    tokenBridgeAddress: state.tokenBridgeAddress,
  }));

  // Hooks
  const { switchChain } = useSwitchNetwork(fromChain?.id);
  const { allowance, fetchAllowance } = useAllowance();
  const { hash: newTxHash, setHash, writeApprove, isLoading: isApprovalLoading } = useApprove();

  // Wagmi
  const { address } = useAccount();
  const {
    isLoading: isWaitingLoading,
    isSuccess: isWaitingSuccess,
    isError: isWaitingError,
  } = useWaitForTransactionReceipt({
    hash: waitingTransaction?.txHash,
    chainId: waitingTransaction?.chainId,
  });

  // Set form allowance
  useEffect(() => {
    setValue("allowance", allowance);
  }, [allowance, setValue]);

  // Set tx hash to trigger useWaitForTransaction
  useEffect(() => {
    newTxHash &&
      setWaitingTransaction({
        txHash: newTxHash,
        chainId: fromChain?.id,
        name: fromChain?.name,
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [newTxHash]);

  // Clear tx waiting when changing account
  useEffect(() => {
    setWaitingTransaction(undefined);
  }, [address]);

  // Refresh allowance after successful tx
  useEffect(() => {
    fetchAllowance();
    if (isWaitingSuccess) {
      toast.success("Token approval successful!");
      setWaitingTransaction(undefined);
      setHash(null);
    }
  }, [isWaitingSuccess, fetchAllowance, setHash]);

  useEffect(() => {
    if (isWaitingError) {
      toast.error("Token approval failed.");
      setWaitingTransaction(undefined);
      setHash(null);
    }
  }, [isWaitingError, setHash]);

  useEffect(() => {
    if (token && allowance && watchAmount && parseUnits(watchAmount, token.decimals) <= allowance) {
      setValue("bridgingAllowed", true);
    } else {
      setValue("bridgingAllowed", false);
    }
  }, [watchAmount, allowance, token, setValue]);

  // Click on approve
  const approveHandler = async () => {
    await switchChain();
    if (token) {
      const amount = getValues("amount");
      const amountToApprove = parseUnits(amount, token.decimals);
      writeApprove(amountToApprove, tokenBridgeAddress);
    }
  };

  return (
    <div className="flex flex-col">
      <button
        id="approve-btn"
        className={classNames("btn btn-primary w-48 rounded-full", {
          "btn-disabled":
            token &&
            (isApprovalLoading ||
              isWaitingLoading ||
              !watchAmount ||
              parseUnits(watchAmount, token.decimals) === BigInt(0) ||
              (allowance && parseUnits(watchAmount, token.decimals) <= allowance) ||
              parseUnits(watchAmount, token.decimals) > parseUnits(watchBalance, token.decimals)),
        })}
        type="button"
        onClick={approveHandler}
      >
        {(isApprovalLoading || isWaitingLoading) && <span className="loading loading-spinner"></span>}
        Approve
      </button>
      {/* <div className="mt-5 text-xs">
        Allowed:{" "}
        {allowance && token ? formatUnits(allowance, token.decimals) : ""}{" "}
        {token?.symbol}
      </div> */}
    </div>
  );
}
