function parseEvent(rawEvent) {
  let eventType = "";
  const dataLines = [];
  for (const line of rawEvent.split("\n")) {
    if (line.startsWith("event:")) {
      eventType = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).replace(/^ /, "")); // strip one optional leading space
    }
  }
  return { type: eventType, data: dataLines.join("\n") };
}

async function* parseSseStream(reader) {
  let buf = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += value;
    const events = buf.split("\n\n");
    buf = events.pop();
    for (const ev of events) {
      if (ev.trim()) yield parseEvent(ev);
    }
  }
}
