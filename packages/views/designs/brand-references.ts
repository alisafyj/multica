// Open Design's curated reference-brand catalogue, ported verbatim for the
// standalone design-system creation page's 从品牌开始 picker. Only public
// facts ship here — display name, website domain, industry bucket — and the
// visuals come from the public favicon service keyed on the domain.

export interface BrandReference {
  /** Brand display name, e.g. "Webflow". */
  name: string;
  /** Website the pick adds as a style-reference link, e.g. "webflow.com". */
  domain: string;
  /** Industry bucket used by the category filter. */
  category: string;
}

/** Upstream's industry buckets, in its order. */
export const BRAND_CATEGORIES = [
  "software",
  "finance",
  "fashion",
  "activewear",
  "beauty",
  "wellness",
  "food",
  "beverage",
  "media",
  "education",
  "electronics",
  "automotive",
  "healthcare",
  "travel",
] as const;

// The buckets are data values, not display copy; these are upstream's own
// zh-CN labels for them. Brand names stay untranslated — proper nouns.
export const BRAND_CATEGORY_LABELS: Record<string, string> = {
  software: "软件",
  finance: "金融",
  fashion: "时尚",
  activewear: "运动服饰",
  beauty: "美妆",
  wellness: "健康养护",
  food: "食品",
  beverage: "饮料",
  media: "媒体",
  education: "教育",
  electronics: "电子产品",
  automotive: "汽车",
  healthcare: "医疗健康",
  travel: "旅行",
};

export function brandCategoryLabel(category: string): string {
  return BRAND_CATEGORY_LABELS[category] ?? category;
}

// Upstream's brand-references.json, in its source order (grouped by bucket).
const SOURCE_BRANDS: BrandReference[] = [
  { name: "HubSpot", domain: "hubspot.com", category: "software" },
  { name: "Shopify", domain: "shopify.com", category: "software" },
  { name: "monday.com", domain: "monday.com", category: "software" },
  { name: "Asana", domain: "asana.com", category: "software" },
  { name: "Webflow", domain: "webflow.com", category: "software" },
  { name: "Slack", domain: "slack.com", category: "software" },
  { name: "Brex", domain: "brex.com", category: "finance" },
  { name: "Wise", domain: "wise.com", category: "finance" },
  { name: "Wealthsimple", domain: "wealthsimple.com", category: "finance" },
  { name: "Ramp", domain: "ramp.com", category: "finance" },
  { name: "Cash App", domain: "cash.app", category: "finance" },
  { name: "Stripe", domain: "stripe.com", category: "finance" },
  { name: "Everlane", domain: "everlane.com", category: "fashion" },
  { name: "ASOS", domain: "asos.com", category: "fashion" },
  { name: "COS", domain: "cos.com", category: "fashion" },
  { name: "Nike", domain: "nike.com", category: "fashion" },
  { name: "New Balance", domain: "newbalance.com", category: "activewear" },
  { name: "Sweaty Betty", domain: "sweatybetty.com", category: "activewear" },
  { name: "Tracksmith", domain: "tracksmith.com", category: "activewear" },
  { name: "Outdoor Voices", domain: "outdoorvoices.com", category: "activewear" },
  { name: "Vuori", domain: "vuoriclothing.com", category: "activewear" },
  { name: "Alo Yoga", domain: "aloyoga.com", category: "activewear" },
  { name: "Glossier", domain: "glossier.com", category: "beauty" },
  { name: "Tatcha", domain: "tatcha.com", category: "beauty" },
  { name: "Summer Fridays", domain: "summerfridays.com", category: "beauty" },
  { name: "Charlotte Tilbury", domain: "charlottetilbury.com", category: "beauty" },
  { name: "Milk Makeup", domain: "milkmakeup.com", category: "beauty" },
  { name: "Supergoop", domain: "supergoop.com", category: "beauty" },
  { name: "Huel", domain: "huel.com", category: "wellness" },
  { name: "Oura", domain: "ouraring.com", category: "wellness" },
  { name: "HelloFresh", domain: "hellofresh.com", category: "food" },
  { name: "Graza", domain: "graza.co", category: "food" },
  { name: "Catalina Crunch", domain: "catalinacrunch.com", category: "food" },
  { name: "Just Salad", domain: "justsalad.com", category: "food" },
  { name: "Clif Bar", domain: "clifbar.com", category: "food" },
  { name: "Cadence", domain: "keepyourcadence.com", category: "beverage" },
  { name: "Liquid Death", domain: "liquiddeath.com", category: "beverage" },
  { name: "Nespresso", domain: "nespresso.com", category: "beverage" },
  { name: "Athletic Brewing", domain: "athleticbrewing.com", category: "beverage" },
  { name: "Spotify", domain: "spotify.com", category: "media" },
  { name: "Vogue", domain: "vogue.com", category: "media" },
  { name: "Hulu", domain: "hulu.com", category: "media" },
  { name: "Wired", domain: "wired.com", category: "media" },
  { name: "Babbel", domain: "babbel.com", category: "education" },
  { name: "Codecademy", domain: "codecademy.com", category: "education" },
  { name: "Samsung", domain: "samsung.com", category: "electronics" },
  { name: "Marshall", domain: "marshallheadphones.com", category: "electronics" },
  { name: "Anker", domain: "anker.com", category: "electronics" },
  { name: "DJI", domain: "dji.com", category: "electronics" },
  { name: "Sonos", domain: "sonos.com", category: "electronics" },
  { name: "Philips", domain: "philips.com", category: "electronics" },
  { name: "Apple", domain: "apple.com", category: "electronics" },
  { name: "Lucid Motors", domain: "lucidmotors.com", category: "automotive" },
  { name: "Honda", domain: "honda.com", category: "automotive" },
  { name: "Porsche", domain: "porsche.com", category: "automotive" },
  { name: "Ford", domain: "ford.com", category: "automotive" },
  { name: "Range Rover", domain: "landrover.com", category: "automotive" },
  { name: "Rivian", domain: "rivian.com", category: "automotive" },
  { name: "BetterHelp", domain: "betterhelp.com", category: "healthcare" },
  { name: "CVS Health", domain: "cvshealth.com", category: "healthcare" },
  { name: "Talkiatry", domain: "talkiatry.com", category: "healthcare" },
  { name: "UnitedHealth", domain: "unitedhealthgroup.com", category: "healthcare" },
  { name: "Walgreens", domain: "walgreens.com", category: "healthcare" },
  { name: "Airbnb", domain: "airbnb.com", category: "travel" },
  { name: "Lonely Planet", domain: "lonelyplanet.com", category: "travel" },
];

// Household names lead the wall; the default tier sits in the middle; smaller
// DTC / niche brands close it. Upstream's fame sets, verbatim.
const TIER_1 = new Set([
  "Apple",
  "Nike",
  "Spotify",
  "Samsung",
  "Airbnb",
  "Stripe",
  "Slack",
  "Shopify",
  "Canva",
  "Porsche",
  "Ford",
  "Honda",
  "Range Rover",
  "H&M",
  "Lululemon",
  "New Balance",
  "Nespresso",
  "Hulu",
  "Vogue",
  "The Economist",
  "Philips",
  "MasterClass",
]);

const TIER_3 = new Set([
  "Sweaty Betty",
  "Tracksmith",
  "Outdoor Voices",
  "Sézane",
  "Summer Fridays",
  "Milk Makeup",
  "Graza",
  "Catalina Crunch",
  "Just Salad",
  "Cava",
  "Cadence",
  "Athletic Brewing",
  "Talkiatry",
  "Architectural Digest",
  "Wealthsimple",
]);

const fameTier = (name: string): number => (TIER_1.has(name) ? 1 : TIER_3.has(name) ? 3 : 2);

/** Fame-ordered wall: famous brands first, source order preserved within a tier. */
export const BRAND_REFERENCES: BrandReference[] = [...SOURCE_BRANDS].sort(
  (a, b) => fameTier(a.name) - fameTier(b.name),
);

// Upstream also pins "The Economist" ahead of this row, but that brand is not
// in its dataset, so the pin is inert there and omitted here: the effective
// quick picks are the first eight of the fame-ordered wall on both sides.
export const QUICK_PICK_BRANDS: BrandReference[] = BRAND_REFERENCES.slice(0, 8);

/** Public favicon for a brand domain, sized for crisp rendering on retina tiles. */
export function brandFaviconUrl(domain: string, size = 64): string {
  return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=${size}`;
}
