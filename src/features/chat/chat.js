const initial = <<< .StoreJSON >>>;

Alpine.store("chat", {
  isStreaming: false,
  _abortController: null,
  totals: initial?.totals ?? null,
  conversationID: initial?.id ?? "",
  title: initial?.title ?? "",
  isBlank: !initial?.id,
  activeTool: "",

  start(ac) {
    this._abortController = ac;
    this.isStreaming = true;
    this.activeTool = "";
  },

  stop() {
    if (this._abortController) {
      this._abortController.abort();
      this._abortController = null;
    }
    this.isStreaming = false;
    this.activeTool = "";
  },

  get statusText() {
    if (this.activeTool) {
      return `[${this.activeTool}]...`;
    }
    return "Responding...";
  },

  setConversation({ id, title, totals }) {
    this.conversationID = id ?? "";
    this.title = title ?? "";
    this.totals = totals ?? null;
    this.isBlank = !id;
  },

  get contextPercent() {
    if (!this.totals || !this.totals.context_window) return 0;
    return Math.min(
      100,
      Math.round(
        (this.totals.context_used_tokens / this.totals.context_window) * 100,
      ),
    );
  },

  get contextNearLimit() {
    const t = this.totals;
    if (!t || !t.context_window || !t.last_input_tokens) return false;
    const remaining = t.context_window - t.context_used_tokens;
    return remaining < t.last_input_tokens * 3;
  },
});

Alpine.data("chat", function () {
  function greeting() {
    const h = new Date().getHours();
    if (h < 12) return "Good morning.";
    if (h < 18) return "Good afternoon.";
    return "Good evening.";
  }

  return {
    _activeReply: null,
    greeting: greeting(),

    init() {
      this.$el.addEventListener("chat-submit", (e) =>
        this.onSubmit(e.detail.text),
      );
      this.$el.addEventListener("chat-stop", () => this.onStop());
    },

    onStop() {
      if (this._activeReply) {
        this._activeReply.cancel();
        this._activeReply = null;
      }
      Alpine.store("chat").stop();
      const textarea = this.$el.querySelector("textarea");
      if (textarea) textarea.focus();
    },

    handleEvent(reply, { type, data }) {
      switch (type) {
        case "text":
          reply.appendText(data);
          break;
        case "tool_use":
        case "tool_result": {
          let tool = null;
          try {
            tool = JSON.parse(data);
          } catch (_) {}
          if (!tool) break;
          if (type === "tool_use") {
            reply.appendTool(tool);
            Alpine.store("chat").activeTool = tool.name;
          } else {
            reply.toolResult(tool);
            Alpine.store("chat").activeTool = "";
          }
          break;
        }
        case "done": {
          let payload = null;
          try {
            payload = JSON.parse(data);
          } catch (_) {}
          const store = Alpine.store("chat");
          const isNew = !store.conversationID;
          if (payload?.conversation_id) {
            store.setConversation({
              id: payload.conversation_id,
              title: payload.title,
              totals: payload.totals,
            });
            if (payload.title) document.title = payload.title;
          }
          if (isNew && store.conversationID) {
            history.replaceState(null, "", "/chat/" + store.conversationID);
          }
          reply.finalise(payload?.exchange);
          this._activeReply = null;
          store.stop();
          break;
        }
      }
    },

    async onSubmit(text) {
      const store = Alpine.store("chat");
      const before = this.$refs.loadingIndicator;

      chatMessages.appendUserMessage(text, this.$refs.messages, before);
      store.isBlank = false;
      Alpine.store("bottomtabs")?.hide();

      const reply = chatMessages.appendAssistantMessage(
        this.$refs.messages,
        before,
      );
      this._activeReply = reply;

      const ac = new AbortController();
      store.start(ac);

      try {
        const res = await fetch("/chat/stream", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            conversation_id: store.conversationID,
            message: text,
          }),
          signal: ac.signal,
        });

        const reader = res.body
          .pipeThrough(new TextDecoderStream())
          .getReader();

        for await (const event of parseSseStream(reader)) {
          this.handleEvent(reply, event);
        }
      } catch (e) {
        if (e.name !== "AbortError") store.stop();
      }
    },
  };
});
