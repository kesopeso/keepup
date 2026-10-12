// Legacy copy is only allowed as a last resort in development.
export function copyWithDevelopmentFallback(text: string): boolean {
  if (process.env.NODE_ENV !== "development" || typeof document === "undefined") {
    return false;
  }

  const previousFocus = document.activeElement;
  const selection = document.getSelection();
  const ranges = selection
    ? Array.from({ length: selection.rangeCount }, (_, index) => selection.getRangeAt(index).cloneRange())
    : [];
  const input = document.createElement("textarea");
  input.value = text;
  input.readOnly = true;
  input.style.position = "fixed";
  input.style.opacity = "0";
  input.style.fontSize = "16px";
  input.style.pointerEvents = "none";

  try {
    document.body.appendChild(input);
    input.focus({ preventScroll: true });
    input.select();
    input.setSelectionRange(0, text.length);
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    input.remove();
    if (previousFocus instanceof HTMLElement) previousFocus.focus({ preventScroll: true });
    if (selection) {
      selection.removeAllRanges();
      for (const range of ranges) selection.addRange(range);
    }
  }
}
