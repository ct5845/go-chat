Alpine.store("tabs", {
  visible: true,
  hide() {
    this.visible = false;
  },
  show() {
    this.visible = true;
  },
});
