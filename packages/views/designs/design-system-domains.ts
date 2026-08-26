/**
 * Curated slug → website host for the built-in design systems, ported from
 * Open Design's `design-system-metadata.ts` (`OFFICIAL_PRESET_DOMAINS`). The
 * bundled packages carry no logo files and their trimmed manifests carry no
 * source URL, so this table is the only way to know which site's favicon
 * represents a system — exactly as upstream: OD renders
 * `https://www.google.com/s2/favicons?domain=<host>&sz=64` from it and falls
 * back to a palette stripe when nothing resolves.
 *
 * Hostnames only, no scheme — the caller builds the favicon URL.
 */
const OFFICIAL_PRESET_DOMAINS: Record<string, string> = {
  airbnb: "airbnb.com",
  airtable: "airtable.com",
  ant: "ant.design",
  apple: "apple.com",
  arc: "arc.net",
  binance: "binance.com",
  bmw: "bmw.com",
  "bmw-m": "bmw-m.com",
  bugatti: "bugatti.com",
  cal: "cal.com",
  canva: "canva.com",
  cisco: "cisco.com",
  claude: "claude.ai",
  clay: "clay.com",
  clickhouse: "clickhouse.com",
  cohere: "cohere.com",
  coinbase: "coinbase.com",
  composio: "composio.dev",
  cursor: "cursor.com",
  discord: "discord.com",
  duolingo: "duolingo.com",
  elevenlabs: "elevenlabs.io",
  expo: "expo.dev",
  ferrari: "ferrari.com",
  figma: "figma.com",
  framer: "framer.com",
  github: "github.com",
  hashicorp: "hashicorp.com",
  huggingface: "huggingface.co",
  ibm: "ibm.com",
  intercom: "intercom.com",
  kraken: "kraken.com",
  lamborghini: "lamborghini.com",
  "linear-app": "linear.app",
  lingo: "lingo.dev",
  loom: "loom.com",
  lovable: "lovable.dev",
  mastercard: "mastercard.com",
  material: "material.io",
  meta: "meta.com",
  minimax: "minimax.io",
  miro: "miro.com",
  mistral: "mistral.ai",
  "mistral-ai": "mistral.ai",
  mongodb: "mongodb.com",
  nike: "nike.com",
  notion: "notion.so",
  nvidia: "nvidia.com",
  ollama: "ollama.com",
  openai: "openai.com",
  "opencode-ai": "opencode.ai",
  perplexity: "perplexity.ai",
  pinterest: "pinterest.com",
  playstation: "playstation.com",
  posthog: "posthog.com",
  raycast: "raycast.com",
  renault: "renault.com",
  replicate: "replicate.com",
  resend: "resend.com",
  revolut: "revolut.com",
  runwayml: "runwayml.com",
  sanity: "sanity.io",
  sentry: "sentry.io",
  shadcn: "ui.shadcn.com",
  shopify: "shopify.com",
  slack: "slack.com",
  spacex: "spacex.com",
  spotify: "spotify.com",
  starbucks: "starbucks.com",
  stripe: "stripe.com",
  supabase: "supabase.com",
  superhuman: "superhuman.com",
  tesla: "tesla.com",
  theverge: "theverge.com",
  "together-ai": "together.ai",
  uber: "uber.com",
  vercel: "vercel.com",
  vodafone: "vodafone.com",
  voltagent: "voltagent.dev",
  warp: "warp.dev",
  webex: "webex.com",
  webflow: "webflow.com",
  wechat: "wechat.com",
  wired: "wired.com",
  wise: "wise.com",
  "x-ai": "x.ai",
  xiaohongshu: "xiaohongshu.com",
  zapier: "zapier.com",
};

/**
 * The host whose favicon represents a built-in system, "" when the catalogue
 * has no entry for it (the row then keeps its icon tile).
 */
export function builtinDesignSystemHost(slug: string): string {
  return OFFICIAL_PRESET_DOMAINS[slug] ?? OFFICIAL_PRESET_DOMAINS[slug.toLowerCase()] ?? "";
}

/**
 * The favicon URL OD renders for a built-in system, "" when no host is known.
 * Served by Google's favicon service; a row whose fetch fails falls back to
 * its icon tile rather than showing a broken image.
 */
export function builtinDesignSystemLogoURL(slug: string): string {
  const host = builtinDesignSystemHost(slug);
  if (!host) return "";
  return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(host)}&sz=64`;
}
