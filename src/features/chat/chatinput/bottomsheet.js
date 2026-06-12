Alpine.data("chatBottomsheet", function () {
  return {
    get display() {
      return Alpine.store("chat").totals?.display ?? {};
    },

    get contextWindowUsage() {
      return Alpine.store("chat").contextPercent;
    },

    get percentOfContextWindow() {
      return Alpine.store("chat").contextPercent + "% used";
    },
  };
});
