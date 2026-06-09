Alpine.store("bottomtabs", {
  visible: true,
  hide() {
    this.visible = false;
  },
  show() {
    this.visible = true;
  },
});

Alpine.data("bottomtabs", () => {
  return {
    visible: Alpine.store("bottomtabs").visible,
  };
});
