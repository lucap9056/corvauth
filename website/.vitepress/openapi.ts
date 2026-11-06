import fs from "node:fs";
import { parse } from "yaml";
import { deploymentModes, isDeploymentMode, type DeploymentMode } from "./modes";

type Spec = Record<string, any>;

export const endpointHeading = /^## (.+?) % (GET|POST|PUT|PATCH|DELETE) (\S+)$/;

export function endpointAnchor(method: string, path: string): string {
  return `${method}${path}`.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/-$/, "");
}

function methodBadge(method: string): string {
  return `<span class="method-badge method-${method.toLowerCase()}">${method}</span>`;
}

function resolveRef(spec: Spec, node: any): any {
  if (!node?.$ref) return node;
  const target = node.$ref
    .replace(/^#\//, "")
    .split("/")
    .reduce((current: any, key: string) => current[key], spec);
  return resolveRef(spec, target);
}

function exampleOf(node: any): unknown {
  return node?.example ?? node?.schema?.example;
}

function bodyLines(contentType: string, example: unknown): string[] {
  const body = typeof example === "string" ? example : JSON.stringify(example, null, 2);
  return [`Content-Type: ${contentType}`, "", body];
}

function credentialHeader(scheme: any): string {
  if (scheme.type === "apiKey" && scheme.in === "cookie") return `Cookie: ${scheme.name}=...`;
  if (scheme.type === "apiKey" && scheme.in === "header") return `${scheme.name}: ...`;
  return "Authorization: Bearer ...";
}

function credentialLabel(scheme: any): string {
  return scheme.type === "apiKey" && scheme.in === "cookie" ? "Cookie" : "Bearer";
}

interface Block {
  label: string;
  lines: string[];
}

function requestBlocks(spec: Spec, method: string, path: string, operation: any): Block[] {
  const parameters = (operation.parameters ?? []).map((p: any) => resolveRef(spec, p));
  const query = parameters
    .filter((p: any) => p.in === "query" && p.required)
    .map((p: any) => `${p.name}=${exampleOf(p) ?? "..."}`)
    .join("&");
  const requestLine = `${method} ${path}${query ? `?${query}` : ""} HTTP/1.1`;
  const headers = parameters
    .filter((p: any) => p.in === "header" && exampleOf(p) !== undefined)
    .map((p: any) => `${p.name}: ${exampleOf(p)}`);

  const requestBody = resolveRef(spec, operation.requestBody);
  const [contentType, media] = Object.entries(requestBody?.content ?? {})[0] ?? [];
  const body = contentType ? bodyLines(contentType, exampleOf(media)) : [];

  const schemes = (operation.security ?? [])
    .map((requirement: Record<string, string[]>) => Object.keys(requirement)[0])
    .filter(Boolean)
    .map((name: string) => spec.components.securitySchemes[name]);

  if (schemes.length === 0) {
    return [{ label: "Request", lines: [requestLine, ...headers, ...body] }];
  }

  const blocks: Block[] = schemes.map((scheme: any) => ({
    label: `Request (${credentialLabel(scheme)})`,
    lines: [requestLine, credentialHeader(scheme), ...headers, ...(requestBody?.required ? body : [])],
  }));
  if (body.length && !requestBody.required) {
    blocks.push({ label: "Request (Body)", lines: [requestLine, ...headers, ...body] });
  }
  return blocks;
}

function responseBlocks(spec: Spec, operation: any): Block[] {
  const [status, rawResponse] = Object.entries(operation.responses).find(([code]) => code.startsWith("2"))!;
  const response = resolveRef(spec, rawResponse);
  const statusLine = `HTTP/1.1 ${status} ${STATUS_TEXT[status] ?? ""}`.trimEnd();
  const defaultHeaders: Record<string, unknown> = Object.fromEntries(
    Object.entries(response.headers ?? {}).map(([name, header]) => [name, exampleOf(resolveRef(spec, header))]),
  );
  const headerLines = (overrides: Record<string, unknown> = {}) =>
    Object.entries({ ...defaultHeaders, ...overrides })
      .filter(([, value]) => value !== undefined && value !== null)
      .map(([name, value]) => `${name}: ${value}`);

  const [contentType, media] = Object.entries<any>(response.content ?? {})[0] ?? [];
  if (!contentType) return [{ label: "Response", lines: [statusLine, ...headerLines()] }];

  const examples: any[] = media.examples ? Object.values(media.examples) : [{ value: exampleOf(media) }];
  return examples.map((example) => ({
    label: example.summary ? `Response (${example.summary})` : "Response",
    lines: [statusLine, ...headerLines(example["x-headers"]), ...bodyLines(contentType, example.value)],
  }));
}

const STATUS_TEXT: Record<string, string> = { "200": "OK", "204": "No Content" };

function codeGroup(blocks: Block[]): string {
  const fences = blocks.map(({ label, lines }) => ["```http [" + label + "]", ...lines, "```"].join("\n"));
  return ["", "::: code-group", ...fences, ":::", ""].join("\n");
}

export function loadSpec(file: string): Spec {
  const spec = parse(fs.readFileSync(file, "utf8"));
  for (const [path, operations] of Object.entries<Record<string, any>>(spec.paths)) {
    for (const [method, operation] of Object.entries(operations)) {
      const modes = operation["x-modes"];
      if (!Array.isArray(modes) || modes.length === 0 || !modes.every(isDeploymentMode)) {
        throw new Error(`${method.toUpperCase()} ${path} needs x-modes from ${deploymentModes.join(", ")}`);
      }
    }
  }
  return spec;
}

function findOperation(spec: Spec, method: string, path: string): any {
  const operation = spec.paths[path]?.[method.toLowerCase()];
  if (!operation) throw new Error(`OpenAPI has no operation for ${method} ${path}`);
  return operation;
}

export function operationModes(spec: Spec, method: string, path: string): DeploymentMode[] {
  return findOperation(spec, method, path)["x-modes"];
}

function endpointCodeGroup(spec: Spec, method: string, path: string): string {
  const operation = findOperation(spec, method, path);
  return codeGroup([...requestBlocks(spec, method, path, operation), ...responseBlocks(spec, operation)]);
}

export function renderEndpoints(spec: Spec, source: string): string {
  const output: string[] = [];
  let pending: string | null = null;
  let inFence = false;

  const flush = () => {
    if (pending) output.push(pending, "", "</div>", "");
    pending = null;
  };

  for (const line of source.split("\n")) {
    if (/^\s*```/.test(line)) inFence = !inFence;
    if (inFence || !line.startsWith("## ")) {
      output.push(line);
      continue;
    }
    flush();
    const match = line.match(endpointHeading);
    if (!match) {
      output.push(line);
      continue;
    }
    const [, name, method, path] = match;
    pending = endpointCodeGroup(spec, method, path);
    output.push(
      `<div class="mode-scoped" data-modes="${operationModes(spec, method, path).join(" ")}">`,
      "",
      `## ${name} {#${endpointAnchor(method, path)}}`,
      "",
      `<div class="endpoint">${methodBadge(method)}<code>${path}</code></div>`,
    );
  }
  flush();
  return output.join("\n");
}
