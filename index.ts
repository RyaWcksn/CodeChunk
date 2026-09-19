import Parser from "tree-sitter";
import TypeScript from "tree-sitter-typescript";
import Go from "tree-sitter-go";
import { readFileSync } from "node:fs";
import { CloudClient, registerEmbeddingFunction, type EmbeddingFunction } from "chromadb";

const tsParser = new Parser();
tsParser.setLanguage(TypeScript.typescript);

const goParser = new Parser();
goParser.setLanguage(Go);

export type Lang = "ts" | "go";

export type Chunk = {
	type: string;
	text: string;
	startLine: number;
	endLine: number;
};

const OLLAMA = process.env.OLLAMA_URL ?? "http://localhost:11434";
const EMBED_MODEL = process.env.EMBED_MODEL ?? "nomic-embed-text";
const COLLECTION = process.env.CHROMA_COLLECTION ?? "code_chunks";

// ponytail: ollama has no SDK dep — fetch directly
export async function embed(texts: string[]): Promise<number[][]> {
	const r = await fetch(`${OLLAMA}/api/embed`, {
		method: "POST",
		headers: { "content-type": "application/json" },
		body: JSON.stringify({ model: EMBED_MODEL, input: texts }),
	});
	if (!r.ok) throw new Error(`ollama ${r.status}: ${await r.text()}`);
	const data = (await r.json()) as { embeddings?: number[][]; embedding?: number[] };
	return data.embeddings ?? (data.embedding ? [data.embedding] : []);
}

// ponytail: register once at module load so chroma can re-instantiate from stored config
class OllamaEF implements EmbeddingFunction {
	static readonly name = "ollama-nomic-embed-text";
	static buildFromConfig(): OllamaEF {
		return new OllamaEF();
	}
	generate(texts: string[]): Promise<number[][]> {
		return embed(texts);
	}
	getConfig() {
		return { model: EMBED_MODEL };
	}
}
registerEmbeddingFunction(OllamaEF.name, OllamaEF);

function parserFor(lang: Lang): Parser {
	return lang === "go" ? goParser : tsParser;
}

// ponytail: skip `export` / `export default` wrapper to expose the real declaration (TS only)
function unwrap(node: Parser.SyntaxNode, lang: Lang): Parser.SyntaxNode {
	if (lang === "ts" && node.type === "export_statement") {
		return node.namedChildren.find((c) => c.type !== "default") ?? node;
	}
	return node;
}

export function chunk(source: string, lang: Lang = "ts"): Chunk[] {
	const root = parserFor(lang).parse(source).rootNode;
	return root.namedChildren.map((node) => {
		const n = unwrap(node, lang);
		return {
			type: n.type,
			text: n.text,
			startLine: n.startPosition.row,
			endLine: n.endPosition.row,
		};
	});
}

export function detectLang(path: string): Lang | null {
	if (/\.go$/.test(path)) return "go";
	if (/\.[cm]?tsx?$/.test(path)) return "ts";
	return null;
}

export function chunkFile(path: string): Chunk[] {
	const lang = detectLang(path);
	if (!lang) throw new Error(`unsupported file: ${path}`);
	return chunk(readFileSync(path, "utf8"), lang);
}

function chromaClient(): CloudClient {
	// ponytail: credentials come from env only — never hardcode (CHROMA_API_KEY/CHROMA_TOKEN, CHROMA_HOST, CHROMA_TENANT, CHROMA_DATABASE)
	return new CloudClient({
		apiKey: process.env.CHROMA_API_KEY ?? process.env.CHROMA_TOKEN,
		host: process.env.CHROMA_HOST,
		tenant: process.env.CHROMA_TENANT ?? "default",
		database: process.env.CHROMA_DATABASE ?? "default",
	});
}

export async function indexFile(path: string): Promise<void> {
	const lang = detectLang(path);
	if (!lang) throw new Error(`unsupported file: ${path}`);
	const chunks = chunkFile(path);
	if (chunks.length === 0) {
		console.error(`no chunks in ${path}`);
		return;
	}
	const client = chromaClient();
	const collection = await client.getOrCreateCollection({
		name: COLLECTION,
		embeddingFunction: new OllamaEF(),
	});
	await collection.upsert({
		ids: chunks.map((c) => `${path}#${c.startLine}-${c.endLine}`),
		documents: chunks.map((c) => c.text),
		metadatas: chunks.map((c) => ({
			file: path,
			lang,
			type: c.type,
			startLine: c.startLine,
			endLine: c.endLine,
		})),
	});
	console.error(`indexed ${chunks.length} chunks from ${path} into ${COLLECTION}`);
}

if (import.meta.main) {
	const cmd = process.argv[2];
	const file = process.argv[3];
	if (cmd === "index" && file) {
		await indexFile(file);
	} else if (file === undefined && cmd && /\.[cm]?tsx?$|\.go$/.test(cmd)) {
		for (const c of chunkFile(cmd)) {
			console.log(`[${c.type}] L${c.startLine + 1}-${c.endLine + 1}`);
		}
	} else {
		console.error("usage: bun run index.ts <file.ts|file.go>   # print chunks");
		console.error("       bun run index.ts index <file>          # chunk + embed + upsert");
		process.exit(1);
	}
}
