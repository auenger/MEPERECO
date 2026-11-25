package metrics

import (
	"regexp"
	"strings"
)

// CostUSDTicksPerUSD is the scale of provider-reported costs: xAI reports
// whole ticks of 1e-10 USD. Declared here rather than imported from pkg/agent
// (which owns the wire-format parsing) so this package keeps no dependency on
// the agent runtime for a physical unit; the two must stay equal.
const CostUSDTicksPerUSD = 10_000_000_000

type ModelPrice struct {
	Provider       string
	Model          string
	InputPerM      float64
	CacheReadPerM  float64
	CacheWritePerM float64
	OutputPerM     float64
}

var modelPrices = map[string]ModelPrice{
	// GPT-5.6 series (Codex `codex` provider). Official rates from OpenAI's
	// GPT-5.6 announcement (openai.com/index/previewing-gpt-5-6-sol). For 5.6+
	// cache read is the 90%-off cached-input rate (0.1x input) and cache write
	// is billed at 1.25x the uncached input rate — unlike earlier OpenAI SKUs,
	// which don't bill cache writes separately. NOTE: Codex's app-server usage
	// stream (0.144.1) does not yet report cache-write tokens separately, so
	// today those tokens fall into plain input and are billed at 1x; the
	// CacheWrite rate below is correct but not yet exercised for Codex.
	"openai:gpt-5.6-sol":   {Provider: "openai", Model: "gpt-5.6-sol", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 6.25, OutputPerM: 30.00},
	"openai:gpt-5.6-terra": {Provider: "openai", Model: "gpt-5.6-terra", InputPerM: 2.50, CacheReadPerM: 0.25, CacheWritePerM: 3.125, OutputPerM: 15.00},
	"openai:gpt-5.6-luna":  {Provider: "openai", Model: "gpt-5.6-luna", InputPerM: 1.00, CacheReadPerM: 0.10, CacheWritePerM: 1.25, OutputPerM: 6.00},
	"openai:gpt-5.5":       {Provider: "openai", Model: "gpt-5.5", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 0.50, OutputPerM: 30.00},
	"openai:gpt-5.4":       {Provider: "openai", Model: "gpt-5.4", InputPerM: 2.50, CacheReadPerM: 0.25, CacheWritePerM: 0.25, OutputPerM: 15.00},
	"openai:gpt-5.4-mini":  {Provider: "openai", Model: "gpt-5.4-mini", InputPerM: 0.75, CacheReadPerM: 0.075, CacheWritePerM: 0.075, OutputPerM: 4.50},
	"openai:gpt-5.3-codex": {Provider: "openai", Model: "gpt-5.3-codex", InputPerM: 1.75, CacheReadPerM: 0.175, CacheWritePerM: 0.175, OutputPerM: 14.00},
	"openai:gpt-5.2-codex": {Provider: "openai", Model: "gpt-5.2-codex", InputPerM: 1.75, CacheReadPerM: 0.175, CacheWritePerM: 0.175, OutputPerM: 14.00},
	// Anthropic's Sonnet 5 launch price is $2 / $10 through 2026-08-31. This
	// static table cannot schedule the published post-intro $3 / $15 change yet,
	// so keep the intro rate here and update the row when catalog support exists.
	"anthropic:claude-sonnet-5":   {Provider: "anthropic", Model: "claude-sonnet-5", InputPerM: 2.00, CacheReadPerM: 0.20, CacheWritePerM: 2.50, OutputPerM: 10.00},
	"anthropic:claude-fable-5":    {Provider: "anthropic", Model: "claude-fable-5", InputPerM: 10.00, CacheReadPerM: 1.00, CacheWritePerM: 12.50, OutputPerM: 50.00},
	"anthropic:claude-opus-5":     {Provider: "anthropic", Model: "claude-opus-5", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 6.25, OutputPerM: 25.00},
	"anthropic:claude-opus-4.8":   {Provider: "anthropic", Model: "claude-opus-4.8", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 6.25, OutputPerM: 25.00},
	"anthropic:claude-opus-4.7":   {Provider: "anthropic", Model: "claude-opus-4.7", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 6.25, OutputPerM: 25.00},
	"anthropic:claude-opus-4.6":   {Provider: "anthropic", Model: "claude-opus-4.6", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 6.25, OutputPerM: 25.00},
	"anthropic:claude-opus-4.5":   {Provider: "anthropic", Model: "claude-opus-4.5", InputPerM: 5.00, CacheReadPerM: 0.50, CacheWritePerM: 6.25, OutputPerM: 25.00},
	"anthropic:claude-sonnet-4.6": {Provider: "anthropic", Model: "claude-sonnet-4.6", InputPerM: 3.00, CacheReadPerM: 0.30, CacheWritePerM: 3.75, OutputPerM: 15.00},
	"anthropic:claude-sonnet-4.5": {Provider: "anthropic", Model: "claude-sonnet-4.5", InputPerM: 3.00, CacheReadPerM: 0.30, CacheWritePerM: 3.75, OutputPerM: 15.00},
	"anthropic:claude-haiku-4.5":  {Provider: "anthropic", Model: "claude-haiku-4.5", InputPerM: 1.00, CacheReadPerM: 0.10, CacheWritePerM: 1.25, OutputPerM: 5.00},
	"deepseek:v4-pro":             {Provider: "deepseek", Model: "v4-pro", InputPerM: 1.74, CacheReadPerM: 0.0145, CacheWritePerM: 1.74, OutputPerM: 3.48},
	"deepseek:v4-flash":           {Provider: "deepseek", Model: "v4-flash", InputPerM: 0.56, CacheReadPerM: 0.0112, CacheWritePerM: 0.56, OutputPerM: 1.12},
	"minimax:m2.7":                {Provider: "minimax", Model: "m2.7", InputPerM: 0.30, CacheReadPerM: 0.06, CacheWritePerM: 0.375, OutputPerM: 1.20},
	"minimax:m2.7-highspeed":      {Provider: "minimax", Model: "m2.7-highspeed", InputPerM: 0.60, CacheReadPerM: 0.06, CacheWritePerM: 0.375, OutputPerM: 2.40},
	"google:gemini-3-flash":       {Provider: "google", Model: "gemini-3-flash", InputPerM: 0.50, CacheReadPerM: 0.05, CacheWritePerM: 0.50, OutputPerM: 3.00},
	"google:gemini-3.1-pro":       {Provider: "google", Model: "gemini-3.1-pro", InputPerM: 2.00, CacheReadPerM: 0.20, CacheWritePerM: 2.00, OutputPerM: 12.00},
	"google:gemini-2.5-pro":       {Provider: "google", Model: "gemini-2.5-pro", InputPerM: 1.25, CacheReadPerM: 0.31, CacheWritePerM: 1.25, OutputPerM: 10.00},
	"google:gemini-2.5-flash":     {Provider: "google", Model: "gemini-2.5-flash", InputPerM: 0.30, CacheReadPerM: 0.03, CacheWritePerM: 0.30, OutputPerM: 2.50},
	// xAI Grok (docs.x.ai/developers/pricing). Short-context tier: xAI bills
	// a request at 2x once its prompt reaches 200K tokens, but a usage record
	// aggregates every model call in a turn, so it cannot say which tier any
	// individual request hit — pricing the standard tier under-estimates a
	// long-context turn by at most 50% instead of over-estimating a short one
	// by 100%. xAI publishes no separate cache-write rate (writes bill as
	// normal input), so CacheWrite mirrors Input. These rows mirror
	// packages/views/runtimes/utils.ts; keep the two tables in sync.
	// `grok-composer-*` ships in the Grok Build catalog but is absent from the
	// price sheet, so it stays unmapped rather than inheriting a guessed rate.
	"xai:grok-4.5":                     {Provider: "xai", Model: "grok-4.5", InputPerM: 2.00, CacheReadPerM: 0.30, CacheWritePerM: 2.00, OutputPerM: 6.00},
	"xai:grok-4.3":                     {Provider: "xai", Model: "grok-4.3", InputPerM: 1.25, CacheReadPerM: 0.20, CacheWritePerM: 1.25, OutputPerM: 2.50},
	"xai:grok-build-0.1":               {Provider: "xai", Model: "grok-build-0.1", InputPerM: 1.00, CacheReadPerM: 0.20, CacheWritePerM: 1.00, OutputPerM: 2.00},
	"xai:grok-4.20-multi-agent-0309":   {Provider: "xai", Model: "grok-4.20-multi-agent-0309", InputPerM: 1.25, CacheReadPerM: 0.20, CacheWritePerM: 1.25, OutputPerM: 2.50},
	"xai:grok-4.20-0309-reasoning":     {Provider: "xai", Model: "grok-4.20-0309-reasoning", InputPerM: 1.25, CacheReadPerM: 0.20, CacheWritePerM: 1.25, OutputPerM: 2.50},
	"xai:grok-4.20-0309-non-reasoning": {Provider: "xai", Model: "grok-4.20-0309-non-reasoning", InputPerM: 1.25, CacheReadPerM: 0.20, CacheWritePerM: 1.25, OutputPerM: 2.50},
	// Previous UI-only reference entries, now shared by server-side estimates.
	"anthropic:claude-opus-4.1":  {Provider: "anthropic", Model: "claude-opus-4.1", InputPerM: 15, OutputPerM: 75, CacheReadPerM: 1.50, CacheWritePerM: 18.75},
	"anthropic:claude-opus-4":    {Provider: "anthropic", Model: "claude-opus-4", InputPerM: 15, OutputPerM: 75, CacheReadPerM: 1.50, CacheWritePerM: 18.75},
	"anthropic:claude-sonnet-4":  {Provider: "anthropic", Model: "claude-sonnet-4", InputPerM: 3, OutputPerM: 15, CacheReadPerM: 0.30, CacheWritePerM: 3.75},
	"anthropic:claude-haiku-3.5": {Provider: "anthropic", Model: "claude-haiku-3.5", InputPerM: 0.80, OutputPerM: 4, CacheReadPerM: 0.08, CacheWritePerM: 1.00},
	"openai:gpt-5-codex":         {Provider: "openai", Model: "gpt-5-codex", InputPerM: 1.25, OutputPerM: 10, CacheReadPerM: 0.125, CacheWritePerM: 1.25},
	"openai:gpt-5-mini":          {Provider: "openai", Model: "gpt-5-mini", InputPerM: 0.25, OutputPerM: 2, CacheReadPerM: 0.025, CacheWritePerM: 0.25},
	"openai:gpt-5-nano":          {Provider: "openai", Model: "gpt-5-nano", InputPerM: 0.05, OutputPerM: 0.40, CacheReadPerM: 0.005, CacheWritePerM: 0.05},
	"openai:gpt-5":               {Provider: "openai", Model: "gpt-5", InputPerM: 1.25, OutputPerM: 10, CacheReadPerM: 0.125, CacheWritePerM: 1.25},
	"openai:o3-mini":             {Provider: "openai", Model: "o3-mini", InputPerM: 1.10, OutputPerM: 4.40, CacheReadPerM: 0.55, CacheWritePerM: 1.10},
	"openai:o3":                  {Provider: "openai", Model: "o3", InputPerM: 2, OutputPerM: 8, CacheReadPerM: 0.50, CacheWritePerM: 2},
	"openai:o4-mini":             {Provider: "openai", Model: "o4-mini", InputPerM: 1.10, OutputPerM: 4.40, CacheReadPerM: 0.275, CacheWritePerM: 1.10},
	"openai:gpt-4o-mini":         {Provider: "openai", Model: "gpt-4o-mini", InputPerM: 0.15, OutputPerM: 0.60, CacheReadPerM: 0.075, CacheWritePerM: 0.15},
	"openai:gpt-4o":              {Provider: "openai", Model: "gpt-4o", InputPerM: 2.50, OutputPerM: 10, CacheReadPerM: 1.25, CacheWritePerM: 2.50},
	"moonshot:kimi-k2.6":         {Provider: "moonshot", Model: "kimi-k2.6", InputPerM: 0.95, OutputPerM: 4.00, CacheReadPerM: 0.16, CacheWritePerM: 0.95},
	"zai:glm-5.1":                {Provider: "zai", Model: "glm-5.1", InputPerM: 1.4, OutputPerM: 4.4, CacheReadPerM: 0.26, CacheWritePerM: 1.4},
	"zai:glm-5":                  {Provider: "zai", Model: "glm-5", InputPerM: 1.0, OutputPerM: 3.2, CacheReadPerM: 0.2, CacheWritePerM: 1.0},
	"zai:glm-5-turbo":            {Provider: "zai", Model: "glm-5-turbo", InputPerM: 1.2, OutputPerM: 4.0, CacheReadPerM: 0.24, CacheWritePerM: 1.2},
	"zai:glm-4.7":                {Provider: "zai", Model: "glm-4.7", InputPerM: 0.6, OutputPerM: 2.2, CacheReadPerM: 0.11, CacheWritePerM: 0.6},
	"zai:glm-4.7-flashx":         {Provider: "zai", Model: "glm-4.7-flashx", InputPerM: 0.07, OutputPerM: 0.4, CacheReadPerM: 0.01, CacheWritePerM: 0.07},
	"zai:glm-4.7-flash":          {Provider: "zai", Model: "glm-4.7-flash", InputPerM: 0, OutputPerM: 0, CacheReadPerM: 0, CacheWritePerM: 0},
	"zai:glm-4.6":                {Provider: "zai", Model: "glm-4.6", InputPerM: 0.6, OutputPerM: 2.2, CacheReadPerM: 0.11, CacheWritePerM: 0.6},
	"zai:glm-4.5":                {Provider: "zai", Model: "glm-4.5", InputPerM: 0.6, OutputPerM: 2.2, CacheReadPerM: 0.11, CacheWritePerM: 0.6},
	"zai:glm-4.5-x":              {Provider: "zai", Model: "glm-4.5-x", InputPerM: 2.2, OutputPerM: 8.9, CacheReadPerM: 0.45, CacheWritePerM: 2.2},
	"zai:glm-4.5-air":            {Provider: "zai", Model: "glm-4.5-air", InputPerM: 0.2, OutputPerM: 1.1, CacheReadPerM: 0.03, CacheWritePerM: 0.2},
	"zai:glm-4.5-airx":           {Provider: "zai", Model: "glm-4.5-airx", InputPerM: 1.1, OutputPerM: 4.5, CacheReadPerM: 0.22, CacheWritePerM: 1.1},
	"zai:glm-4.5-flash":          {Provider: "zai", Model: "glm-4.5-flash", InputPerM: 0, OutputPerM: 0, CacheReadPerM: 0, CacheWritePerM: 0},
	"cursor:auto":                {Provider: "cursor", Model: "auto", InputPerM: 1.25, OutputPerM: 6, CacheReadPerM: 0.25, CacheWritePerM: 0},
	"cursor:composer-2.5-fast":   {Provider: "cursor", Model: "composer-2.5-fast", InputPerM: 3, OutputPerM: 15, CacheReadPerM: 0.5, CacheWritePerM: 0},
	"cursor:composer-2.5":        {Provider: "cursor", Model: "composer-2.5", InputPerM: 0.5, OutputPerM: 2.5, CacheReadPerM: 0.2, CacheWritePerM: 0},
	"cursor:composer-2-fast":     {Provider: "cursor", Model: "composer-2-fast", InputPerM: 1.5, OutputPerM: 7.5, CacheReadPerM: 0.35, CacheWritePerM: 0},
	"cursor:composer-2":          {Provider: "cursor", Model: "composer-2", InputPerM: 0.5, OutputPerM: 2.5, CacheReadPerM: 0.2, CacheWritePerM: 0},
	"cursor:composer-1.5":        {Provider: "cursor", Model: "composer-1.5", InputPerM: 3.5, OutputPerM: 17.5, CacheReadPerM: 0.35, CacheWritePerM: 0},
	"cursor:composer-1":          {Provider: "cursor", Model: "composer-1", InputPerM: 1.25, OutputPerM: 10, CacheReadPerM: 0.125, CacheWritePerM: 0},
	"cursor:cursor":              {Provider: "cursor", Model: "cursor", InputPerM: 3, OutputPerM: 15, CacheReadPerM: 0.5, CacheWritePerM: 0},
}

var modelAliasRules = []struct {
	re       *regexp.Regexp
	priceKey string
}{
	// Anchored exact-match: the effort is carried in a separate field, so the
	// model id is the bare slug. Anchoring to `$` keeps unknown variants
	// (`gpt-5.6-luna-pro`, `gpt-5.6-luna/x`) out of these rows. The `.` is a
	// LITERAL dot, not the `[.-]` class the older rows use — the real Codex
	// slug is always dotted (`gpt-5.6-luna`), and the frontend resolver in
	// utils.ts does NOT dash-normalize, so a dashed `gpt-5-6-luna` must surface
	// as unmapped on both sides rather than silently borrowing a tier here.
	{regexp.MustCompile(`(^|/|:)gpt-5\.6-sol$`), "openai:gpt-5.6-sol"},
	{regexp.MustCompile(`(^|/|:)gpt-5\.6-terra$`), "openai:gpt-5.6-terra"},
	{regexp.MustCompile(`(^|/|:)gpt-5\.6-luna$`), "openai:gpt-5.6-luna"},
	{regexp.MustCompile(`(^|/|:)gpt-5[.-]5$|^gpt-5-5$`), "openai:gpt-5.5"},
	{regexp.MustCompile(`(^|/|:)gpt-5[.-]4($|-2026-03-05|-xhigh)`), "openai:gpt-5.4"},
	{regexp.MustCompile(`(^|/|:)gpt-5[.-]4-mini($|[^a-z0-9])`), "openai:gpt-5.4-mini"},
	{regexp.MustCompile(`(^|/|:)gpt-5[.-]3-codex$`), "openai:gpt-5.3-codex"},
	{regexp.MustCompile(`(^|/|:)gpt-5[.-]2-codex$`), "openai:gpt-5.2-codex"},
	{regexp.MustCompile(`claude-sonnet-5|claude-5-sonnet`), "anthropic:claude-sonnet-5"},
	{regexp.MustCompile(`claude-fable-5`), "anthropic:claude-fable-5"},
	{regexp.MustCompile(`claude-opus-5`), "anthropic:claude-opus-5"},
	{regexp.MustCompile(`claude-opus-4[-.]8`), "anthropic:claude-opus-4.8"},
	{regexp.MustCompile(`claude-opus-4[-.]7`), "anthropic:claude-opus-4.7"},
	{regexp.MustCompile(`claude-opus-4[-.]6`), "anthropic:claude-opus-4.6"},
	{regexp.MustCompile(`claude-opus-4[-.]5`), "anthropic:claude-opus-4.5"},
	{regexp.MustCompile(`claude-sonnet-4[-.]6|claude-4[-.]6-sonnet`), "anthropic:claude-sonnet-4.6"},
	{regexp.MustCompile(`claude-sonnet-4[-.]5|claude-4[-.]5-sonnet`), "anthropic:claude-sonnet-4.5"},
	{regexp.MustCompile(`claude-haiku-4[-.]5`), "anthropic:claude-haiku-4.5"},
	{regexp.MustCompile(`deepseek-v4-pro`), "deepseek:v4-pro"},
	{regexp.MustCompile(`deepseek-v4-flash|^deepseek-chat$|^deepseek-reasoner$`), "deepseek:v4-flash"},
	{regexp.MustCompile(`minimax-m2[.]7.*highspeed|highspeed.*minimax-m2[.]7`), "minimax:m2.7-highspeed"},
	{regexp.MustCompile(`minimax-m2[.]7`), "minimax:m2.7"},
	{regexp.MustCompile(`gemini-3-flash`), "google:gemini-3-flash"},
	{regexp.MustCompile(`gemini-3[.]1-pro`), "google:gemini-3.1-pro"},
	{regexp.MustCompile(`gemini-2[.]5-pro`), "google:gemini-2.5-pro"},
	{regexp.MustCompile(`gemini-2[.]5-flash`), "google:gemini-2.5-flash"},
	// Anchored exact-match, dotted spelling only — same rule as the gpt-5.6
	// rows above. The frontend resolver does not dash-normalize non-Anthropic
	// ids, so a dashed `grok-4-5` must surface as unmapped on both sides
	// rather than silently borrowing a tier here.
	{regexp.MustCompile(`(^|/|:)grok-4\.5$`), "xai:grok-4.5"},
	{regexp.MustCompile(`(^|/|:)grok-4\.3$`), "xai:grok-4.3"},
	{regexp.MustCompile(`(^|/|:)grok-build-0\.1$`), "xai:grok-build-0.1"},
	{regexp.MustCompile(`(^|/|:)grok-4\.20-multi-agent-0309$`), "xai:grok-4.20-multi-agent-0309"},
	{regexp.MustCompile(`(^|/|:)grok-4\.20-0309-reasoning$`), "xai:grok-4.20-0309-reasoning"},
	{regexp.MustCompile(`(^|/|:)grok-4\.20-0309-non-reasoning$`), "xai:grok-4.20-0309-non-reasoning"},
}

func PriceForModelAlias(model string) (ModelPrice, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, rule := range modelAliasRules {
		if rule.re.MatchString(model) {
			price, ok := modelPrices[rule.priceKey]
			return price, ok
		}
	}
	return ModelPrice{}, false
}

func tokenCostUSD(tokens int64, pricePerM float64) float64 {
	if tokens <= 0 || pricePerM <= 0 {
		return 0
	}
	return float64(tokens) * pricePerM / 1_000_000
}
