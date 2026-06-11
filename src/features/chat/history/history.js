const confirmDialog = document.getElementById("confirm-delete");
let deleteButton = null;

confirmDialog.addEventListener("command", (event) => {
  if (event.command === "show-modal") {
    deleteButton = event.source;
    // returnValue persists across opens; reset so a light dismiss
    // (esc/backdrop) after an earlier confirm doesn't read as "delete"
    confirmDialog.returnValue = "";
  }
});

confirmDialog.addEventListener("close", async () => {
  const button = deleteButton;
  deleteButton = null;
  if (confirmDialog.returnValue !== "delete" || !button) return;

  const response = await fetch("/chat/" + button.dataset.id, {
    method: "DELETE",
  });
  if (!response.ok) {
    console.error("Failed to delete conversation", response.status);
    return;
  }
  button.closest("li").remove();
  const nextMenu = document.querySelector(".conversation-menu");
  if (nextMenu) {
    nextMenu.focus();
  } else {
    // last item deleted: reload so the server renders the empty state
    location.reload();
  }
});
