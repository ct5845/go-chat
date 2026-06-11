Alpine.data("chatBottomsheet", function () {
  return {
    get _totals() {
      return Alpine.store("chat").totals;
    },

    get totalCost() {
      const t = this._totals;
      if (!t) return "$0.000000";
      return t.cost_usd < 0.000001 ? "<$0.000001" : "$" + t.cost_usd.toFixed(6);
    },

    get totalMessages() {
      const count = this._totals?.exchange_count ?? 0;
      return count === 1 ? "1 message" : `${count} messages`;
    },

    get contextWindow() {
      return (this._totals?.context_window ?? 0).toLocaleString();
    },

    get contextWindowUsage() {
      return Alpine.store("chat").contextPercent;
    },

    get percentOfContextWindow() {
      return Alpine.store("chat").contextPercent + "% used";
    },

    get contextUsedTokens() {
      return (this._totals?.context_used_tokens ?? 0).toLocaleString();
    },

    get totalInputTokens() {
      return (this._totals?.input_tokens ?? 0).toLocaleString();
    },

    get totalOutputTokens() {
      return (this._totals?.output_tokens ?? 0).toLocaleString();
    },

    get totalCacheRead() {
      return (this._totals?.cache_read_input_tokens ?? 0).toLocaleString();
    },

    get averageResponseTime() {
      const ms = this._totals?.avg_response_ms;
      if (!ms) return "—";
      return ms >= 1000 ? (ms / 1000).toFixed(1) + " s" : ms + " ms";
    },
  };
});
