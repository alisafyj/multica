// @vitest-environment node

import { afterEach, describe, expect, it } from "vitest";
import { useCustomPricingStore } from "@multica/core/runtimes/custom-pricing-store";
import { collectUnmappedModels, summarizeTaskUsage, summarizeTaskUsageAcross, type Priceable } from "./utils";

const unknown: Priceable = {
  provider: "r35-test",
  model: "unpriced-model",
  input_tokens: 1_000_000,
  output_tokens: 0,
  cache_read_tokens: 1_000_000,
  cache_write_tokens: 0,
};
const known: Priceable = { ...unknown, provider: "anthropic", model: "claude-opus-5" };

afterEach(() => useCustomPricingStore.setState({ pricings: {} }));

describe("task usage amount completeness", () => {
  it("keeps unpriced tokens without claiming a complete zero amount", () => {
    expect(summarizeTaskUsage([unknown])).toMatchObject({
      tokens: 2_000_000, cost: 0, costComplete: false,
      cacheSavings: 0, cacheSavingsComplete: false,
    });
  });

  it.each(["input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens"] as const)(
    "detects missing pricing for %s alone",
    (bucket) => {
      const slice = { ...unknown, input_tokens: 0, cache_read_tokens: 0, [bucket]: 1 };
      expect(summarizeTaskUsage([slice])).toMatchObject({
        costComplete: false, cacheSavingsComplete: bucket !== "cache_read_tokens",
      });
    },
  );

  it("does not confuse provider-uncosted tokens with a missing maintained rate", () => {
    expect(summarizeTaskUsage([{ ...known, uncosted_input_tokens: 1_000_000,
      uncosted_output_tokens: 0, uncosted_cache_read_tokens: 1_000_000, uncosted_cache_write_tokens: 0,
    }])).toMatchObject({ cost: 5.5, costComplete: true, cacheSavings: 4.5, cacheSavingsComplete: true });
  });

  it("keeps a known subtotal but marks mixed runs incomplete", () => {
    expect(summarizeTaskUsageAcross([[known], undefined, [unknown]])).toMatchObject({
      tokens: 4_000_000, cost: 5.5, costComplete: false,
      cacheSavings: 4.5, cacheSavingsComplete: false,
    });
  });

  it("treats recorded provider amounts separately from rate-based cache savings", () => {
    expect(summarizeTaskUsage([{ ...unknown, cost_usd_ticks: 30_000_000_000 }])).toMatchObject({
      cost: 3, costComplete: true, cacheSavings: 0, cacheSavingsComplete: false,
    });
  });

  it("does not hide a still-unpriced split behind a positive provider amount", () => {
    expect(summarizeTaskUsage([{ ...unknown, cost_usd_ticks: 30_000_000_000,
      uncosted_input_tokens: 10, uncosted_output_tokens: 0,
      uncosted_cache_read_tokens: 0, uncosted_cache_write_tokens: 0,
    }])).toMatchObject({ cost: 3, costComplete: false });
  });

  it("preserves an explicitly fully costed zero split", () => {
    expect(summarizeTaskUsage([{ ...unknown, cost_usd_ticks: 0,
      uncosted_input_tokens: 0, uncosted_output_tokens: 0,
      uncosted_cache_read_tokens: 0, uncosted_cache_write_tokens: 0,
    }])).toMatchObject({ cost: 0, costComplete: true, cacheSavingsComplete: false });
  });

  it("does not warn that fully reported zero-cost tokens were excluded", () => {
    const costed = { ...unknown, cost_usd_ticks: 0, uncosted_input_tokens: 0,
      uncosted_output_tokens: 0, uncosted_cache_read_tokens: 0, uncosted_cache_write_tokens: 0 };
    expect(collectUnmappedModels([costed])).toEqual([]);
    expect(collectUnmappedModels([{ ...costed, uncosted_output_tokens: 1 }])).toEqual(["r35-test/unpriced-model"]);
  });

  it("does not ask for pricing when no tokens were consumed", () => {
    expect(collectUnmappedModels([{ ...unknown, input_tokens: 0, cache_read_tokens: 0 }])).toEqual([]);
  });

  it("does not invent missing monetary amounts for zero tokens", () => {
    expect(summarizeTaskUsage([{ ...unknown, input_tokens: 0, cache_read_tokens: 0 }])).toMatchObject({
      tokens: 0, cost: 0, costComplete: true, cacheSavings: 0, cacheSavingsComplete: true,
    });
  });

  it("recognizes a custom zero rate and changes back when the rate is removed", () => {
    useCustomPricingStore.getState().setCustomPricing("r35-test/unpriced-model", {
      input: 0, output: 0, cacheRead: 0, cacheWrite: 0,
    });
    expect(summarizeTaskUsage([unknown])).toMatchObject({
      cost: 0, costComplete: true, cacheSavings: 0, cacheSavingsComplete: true,
    });
    useCustomPricingStore.setState({ pricings: {} });
    expect(summarizeTaskUsage([unknown])).toMatchObject({ costComplete: false, cacheSavingsComplete: false });
  });

  it("preserves the absence of usage", () => {
    expect(summarizeTaskUsage(undefined)).toBeNull();
    expect(summarizeTaskUsageAcross([undefined, []])).toBeNull();
  });
});
