Alpine.store("debug", {
  value: localStorage.getItem("debug") === "true",

  toggle() {
    const enabled = localStorage.getItem("debug") !== "true";
    localStorage.setItem("debug", enabled);
    this.value = enabled;
    return enabled;
  },
});

Alpine.data("webcli", () => {
  const pages = [
    { name: "home", path: "/" },
    { name: "chat", path: "/chat" },
    { name: "chat/history", path: "/chat/history" },
  ];

  return {
    input: "",
    lines: [],
    selected: 0,

    init() {
      this._onKeydown = (e) => {
        if (e.ctrlKey && e.key === "k") {
          e.preventDefault();
          this.$refs.dialog.showModal();
        }
      };
      window.addEventListener("keydown", this._onKeydown);
      this.$watch("input", () => (this.selected = 0));
    },

    destroy() {
      window.removeEventListener("keydown", this._onKeydown);
    },

    get suggestions() {
      const text = this.input.trimStart();
      if (text.startsWith("goto")) {
        const partial = text.slice(4).trimStart();
        return pages
          .filter((p) => p.name.startsWith(partial))
          .map((p) => ({
            label: "goto " + p.name,
            complete: "goto " + p.name,
          }));
      }
      return [
        { label: "goto", complete: "goto " },
        { label: "debug", complete: "debug" },
      ].filter((c) => c.label.startsWith(text));
    },

    selectNext() {
      if (this.suggestions.length === 0) return;
      this.selected = (this.selected + 1) % this.suggestions.length;
    },

    selectPrev() {
      if (this.suggestions.length === 0) return;
      this.selected =
        (this.selected - 1 + this.suggestions.length) % this.suggestions.length;
    },

    complete(index = this.selected) {
      const suggestion = this.suggestions[index];
      if (suggestion) this.input = suggestion.complete;
    },

    echo(line) {
      this.lines.push(line);
      this.$nextTick(() => {
        this.$refs.output.scrollTop = this.$refs.output.scrollHeight;
      });
    },

    run() {
      const cmd = this.input.trim();
      if (!cmd) return;
      this.input = "";
      this.echo("> " + cmd);

      const [name, ...args] = cmd.split(/\s+/);
      if (name === "debug") {
        this.echo("debug " + (Alpine.store("debug").toggle() ? "on" : "off"));
      } else if (name === "goto") {
        const page = pages.find((p) => p.name === args.join(" "));
        if (page) {
          window.location.href = page.path;
        } else {
          this.echo("unknown page: " + args.join(" "));
        }
      } else {
        this.echo("unknown command: " + name);
      }
    },

    onClose() {
      this.input = "";
      this.lines = [];
    },
  };
});
