import katex from "katex";
import "katex/dist/katex.min.css";

/**
 * Renders inline `$...$` LaTeX (via KaTeX) plus **bold** / *italic* markdown
 * within a single line of text. Content is admin-authored (question bank),
 * not user-submitted — safe to inject KaTeX's own sanitized markup.
 */
function renderInline(text: string, keyPrefix: string): React.ReactNode[] {
  const dollarParts = text.split(/(\$[^$]+\$)/g).filter((part) => part !== "");

  return dollarParts.flatMap((part, i) => {
    const dollarKey = `${keyPrefix}-d${i}`;
    if (part.startsWith("$") && part.endsWith("$") && part.length > 1) {
      const html = katex.renderToString(part.slice(1, -1), {
        throwOnError: false,
        displayMode: false,
      });
      // eslint-disable-next-line react/no-danger
      return [<span key={dollarKey} dangerouslySetInnerHTML={{ __html: html }} />];
    }

    // Markers must hug their content (no adjacent whitespace) so a literal
    // multiplication like "3 * 4 * 5" isn't misread as italics.
    const tokens = part.split(/(\*\*(?!\s)[^*]+(?<!\s)\*\*|\*(?!\s)[^*]+(?<!\s)\*)/g).filter((token) => token !== "");
    return tokens.map((token, j) => {
      const tokenKey = `${dollarKey}-t${j}`;
      if (token.startsWith("**") && token.endsWith("**") && token.length > 3) {
        return <strong key={tokenKey}>{token.slice(2, -2)}</strong>;
      }
      if (token.startsWith("*") && token.endsWith("*") && token.length > 1) {
        return <em key={tokenKey}>{token.slice(1, -1)}</em>;
      }
      return <span key={tokenKey}>{token}</span>;
    });
  });
}

/**
 * Renders question/explanation text: $...$ LaTeX, **bold**, *italic*,
 * "- " / "1. " list lines, and "![alt](url)" images on their own line.
 * Plain text with none of these markers renders exactly as before — every
 * existing question in the bank is unaffected.
 */
export function LatexText({ text, className }: { text: string; className?: string }) {
  const lines = text.split("\n");
  const blocks: React.ReactNode[] = [];
  let listBuffer: { type: "ul" | "ol"; text: string; raw: string }[] = [];

  // A single "- " or "1. " line is ambiguous with a negative number or a
  // decimal-ish sentence start (e.g. "- 5 adalah akar..."), so we only
  // commit to real list markup once 2+ consecutive lines match — otherwise
  // the lone line falls back to its literal text, dash/number and all.
  const flushList = () => {
    if (listBuffer.length === 0) return;
    if (listBuffer.length === 1) {
      const only = listBuffer[0];
      blocks.push(<span key={`block-${blocks.length}`}>{renderInline(only.raw, `single${blocks.length}`)}</span>);
      listBuffer = [];
      return;
    }
    const type = listBuffer[0].type;
    const items = listBuffer.map((item, i) => <li key={i}>{renderInline(item.text, `list${blocks.length}-${i}`)}</li>);
    blocks.push(
      type === "ul" ? (
        <ul key={`block-${blocks.length}`} className="list-disc space-y-0.5 pl-5">
          {items}
        </ul>
      ) : (
        <ol key={`block-${blocks.length}`} className="list-decimal space-y-0.5 pl-5">
          {items}
        </ol>
      ),
    );
    listBuffer = [];
  };

  lines.forEach((line, i) => {
    const imageMatch = line.match(/^!\[([^\]]*)\]\(([^)]+)\)$/);
    const bulletMatch = line.match(/^-\s+(.*)$/);
    const numberedMatch = line.match(/^\d+\.\s+(.*)$/);

    if (imageMatch) {
      flushList();
      // eslint-disable-next-line @next/next/no-img-element -- admin-authored data: URLs, not a static asset
      blocks.push(<img key={`img-${i}`} src={imageMatch[2]} alt={imageMatch[1]} className="my-1 max-w-full rounded-lg border border-border" />);
      return;
    }
    if (bulletMatch) {
      listBuffer.push({ type: "ul", text: bulletMatch[1], raw: line });
      return;
    }
    if (numberedMatch) {
      listBuffer.push({ type: "ol", text: numberedMatch[1], raw: line });
      return;
    }

    flushList();
    if (line !== "") blocks.push(<span key={`line-${i}`}>{renderInline(line, `line${i}`)}</span>);
    if (i < lines.length - 1) blocks.push(<br key={`br-${i}`} />);
  });
  flushList();

  return <span className={className}>{blocks}</span>;
}
