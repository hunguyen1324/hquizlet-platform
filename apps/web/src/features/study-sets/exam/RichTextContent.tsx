import React from "react";

function safeContent(value: string): React.ReactNode {
 if (typeof DOMParser === "undefined" || !/<[a-z]/i.test(value)) return value;
 const document = new DOMParser().parseFromString(value, "text/html");
 const allowed = new Set(["p", "br", "b", "strong", "em", "i", "u", "ul", "ol", "li", "table", "tbody", "thead", "tr", "td", "th", "blockquote", "span", "div", "sup", "sub"]);
 const blocked = new Set(["script", "style", "iframe", "object", "embed", "audio", "video", "source", "link", "meta"]);
 const visit = (node: Node, key: number): React.ReactNode => {
   if (node.nodeType === Node.TEXT_NODE) return node.textContent;
   if (node.nodeType !== Node.ELEMENT_NODE) return null;
   const tag = (node as Element).tagName.toLowerCase();
   if (blocked.has(tag)) return null;
   const children = Array.from(node.childNodes).map(visit);
   if (tag === "img") return (node as Element).getAttribute("alt") ?? "";
   return allowed.has(tag) ? React.createElement(tag, { key }, tag === "br" ? undefined : children) : children;
 };
 return Array.from(document.body.childNodes).map(visit);
}
export function RichTextContent({ value, className = "" }: { value?: string | null; className?: string }) {
 const content = React.useMemo(() => safeContent((value ?? "").trim()), [value]);
 return <div className={`whitespace-pre-wrap ${className}`.trim()}>{content}</div>;
}
