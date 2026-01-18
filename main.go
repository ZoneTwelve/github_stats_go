package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/go-github/v58/github"
	"github.com/joho/godotenv"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// UserStats represents GitHub user statistics
type UserStats struct {
	Name            string `json:"name"`
	Login           string `json:"login"`
	PublicRepos     int    `json:"public_repos"`
	Followers       int    `json:"followers"`
	Following       int    `json:"following"`
	TotalStars      int    `json:"total_stars"`
	TotalCommits    int    `json:"total_commits"`
	TotalPRs        int    `json:"total_prs"`
	TotalIssues     int    `json:"total_issues"`
	ProfileImageURL string `json:"avatar_url"`
}

// Global template variables
var (
	templates   *template.Template
	templateDir = "templates"
)

// TemplateData represents data passed to SVG templates
type TemplateData struct {
	// Card dimensions
	CardWidth          int
	CardHeight         int
	CardWidthMinus4    int
	CardHeightMinus4   int
	BorderRadius       int
	BorderRadiusMinus2 int
	BorderWidth        int
	Padding            int
	IconOffset         int
	ColSpacing         int
	ColSpacingMult     int
	ColSpacingTriple   int

	// User info
	UserName        string
	Username        string
	Name            string
	CustomTitle     string
	ProfileImageURL string

	// Statistics
	PublicRepos  int
	Followers    int
	Following    int
	TotalStars   int
	TotalCommits int
	TotalPRs     int
	TotalIssues  int

	// Progress bars
	ProgressBarWidth     int
	CommitsProgressWidth int
	PRsProgressWidth     int
	IssuesProgressWidth  int
	StatsYStart          int
	ContribYStart        int
	AdditionalStatsY     int
	FooterY              int

	// Colors
	TitleColor        string
	SubtitleColor     string
	TextColor         string
	SecondaryText     string
	IconColor         string
	StarColor         string
	AccentColor       string
	AccentColorAlt    string
	SectionTitleColor string
	FooterColor       string
	BorderColor       string
	ProgressBg        string
	RankColor         string
	RankBg            string
	RankBorder        string
	ShadowColor       string

	// Background colors
	BGColor       string
	CardBg        string
	CardBgAlt     string
	GradientStart string
	GradientEnd   string

	// Fonts
	FontFamily   string
	TitleSize    int
	SubtitleSize int

	// Theme options
	ShowIcons         bool
	HideBorder        bool
	HideTitle         bool
	ShowRank          bool
	CustomTitleShow   bool
	ShowFooter        bool
	DisableAnimations bool

	// Show/hide individual stats
	HideRepo       bool
	HideFollowers  bool
	HideFollowing  bool
	HideStars      bool
	HideCommits    bool
	HidePRs        bool
	HideIssues     bool
	ShowAdditional bool
	ShowIconsCond  bool

	// Additional info
	Rank          string
	RankX         int
	RankY         int
	GeneratedDate string
	FooterText    string
}

// loadTemplates loads all SVG templates from the templates directory
func loadTemplates() error {
	tmpl := template.New("")

	// Parse all SVG files in templates directory
	files, err := filepath.Glob(filepath.Join(templateDir, "*.svg"))
	if err != nil {
		return fmt.Errorf("failed to glob template files: %v", err)
	}

	for _, file := range files {
		name := filepath.Base(file)
		content, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %v", file, err)
		}

		// Parse template with the file content
		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %v", name, err)
		}
	}

	templates = tmpl
	return nil
}

// renderSVGTemplate renders an SVG template with the provided data
func renderSVGTemplate(templateName string, data TemplateData) (string, error) {
	if templates == nil {
		return "", fmt.Errorf("templates not loaded")
	}

	var result strings.Builder
	err := templates.ExecuteTemplate(&result, templateName, data)
	return result.String(), err
}

// SVGConfig represents SVG customization parameters
type SVGConfig struct {
	// Layout
	CardWidth  int `json:"card_width"`
	CardHeight int `json:"card_height"`

	// Colors
	TitleColor  string `json:"title_color"`
	TextColor   string `json:"text_color"`
	IconColor   string `json:"icon_color"`
	BorderColor string `json:"border_color"`
	BGColor     string `json:"bg_color"`

	// Typography
	TitleSize  int    `json:"title_size"`
	TextSize   int    `json:"text_size"`
	FontFamily string `json:"font_family"`

	// Features
	ShowIcons  bool `json:"show_icons"`
	HideBorder bool `json:"hide_border"`
	HideTitle  bool `json:"hide_title"`
	HideRank   bool `json:"hide_rank"`

	// Theme
	Theme string `json:"theme"`

	// Layout options
	Layout   string `json:"layout"`    // "default", "compact"
	RankIcon string `json:"rank_icon"` // "default", "badge"

	// Content Options
	CustomTitle string   `json:"custom_title"`
	Hide        []string `json:"hide"`
	Show        []string `json:"show"`

	// Advanced
	Locale            string `json:"locale"`
	DisableAnimations bool   `json:"disable_animations"`
	BorderRadius      int    `json:"border_radius"`
}

// Repo represents a GitHub repository
type Repo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stargazers_count"`
	Language    string `json:"language"`
	HTMLURL     string `json:"html_url"`
}

// Language represents programming language statistics
type Language struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
	Color string `json:"color"`
}

type UserLanguages struct {
	Login               string             `json:"login"`
	Languages           map[string]int     `json:"languages"`
	TotalBytes          int                `json:"total_bytes"`
	LanguagePercentages map[string]float64 `json:"percentages"`
}

var (
	githubToken  = os.Getenv("GITHUB_TOKEN")
	port         = getEnv("PORT", "3000")
	cacheSeconds = getEnv("CACHE_SECONDS", "86400") // default 1 day
)

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Default themes
var themes = map[string]map[string]string{
	"default": {
		"bg":      "#0d1117",
		"text":    "#f0f6fc",
		"title":   "#f0f6fc",
		"icon":    "#58a6ff",
		"border":  "#161b22",
		"card_bg": "#161b22",
	},
	"dark": {
		"bg":      "#000000",
		"text":    "#ffffff",
		"title":   "#ffffff",
		"icon":    "#58a6ff",
		"border":  "#333333",
		"card_bg": "#1a1a1a",
	},
	"radical": {
		"bg":      "#fe428e",
		"text":    "#f8f8f2",
		"title":   "#f8f8f2",
		"icon":    "#f8f8f2",
		"border":  "#ff006e",
		"card_bg": "#0f0e17",
	},
	"merko": {
		"bg":      "#abd200",
		"text":    "#040300",
		"title":   "#040300",
		"icon":    "#040300",
		"border":  "#a1de93",
		"card_bg": "#b8e986",
	},
	"gruvbox": {
		"bg":      "#282828",
		"text":    "#ebdbb2",
		"title":   "#ebdbb2",
		"icon":    "#fabd2f",
		"border":  "#504945",
		"card_bg": "#3c3836",
	},
	"algolia": {
		"bg":      "#2c333b",
		"text":    "#ffffff",
		"title":   "#ffffff",
		"icon":    "#00a1ff",
		"border":  "#3e4c5e",
		"card_bg": "#2d3742",
	},
	"highcontrast": {
		"bg":      "#000000",
		"text":    "#ffffff",
		"title":   "#ffffff",
		"icon":    "#ffff00",
		"border":  "#ffffff",
		"card_bg": "#1a1a1a",
	},
}

// parseSVGConfig extracts SVG customization parameters from HTTP request
func parseSVGConfig(r *http.Request) SVGConfig {
	config := SVGConfig{
		CardWidth:    500,
		CardHeight:   300,
		TitleSize:    20,
		TextSize:     14,
		FontFamily:   "Segoe UI, Arial, sans-serif",
		ShowIcons:    false,
		HideBorder:   false,
		HideTitle:    false,
		HideRank:     false,
		Theme:        "default",
		Layout:       "default",
		RankIcon:     "default",
		Locale:       "en",
		BorderRadius: 12,
	}

	// Parse query parameters
	query := r.URL.Query()

	// Colors
	if titleColor := query.Get("title_color"); titleColor != "" {
		config.TitleColor = titleColor
	}
	if textColor := query.Get("text_color"); textColor != "" {
		config.TextColor = textColor
	}
	if iconColor := query.Get("icon_color"); iconColor != "" {
		config.IconColor = iconColor
	}
	if bgColor := query.Get("bg_color"); bgColor != "" {
		config.BGColor = bgColor
	}
	if borderColor := query.Get("border_color"); borderColor != "" {
		config.BorderColor = borderColor
	}

	// Theme
	if theme := query.Get("theme"); theme != "" {
		config.Theme = theme
	}

	// Layout options
	if query.Get("hide_title") == "true" {
		config.HideTitle = true
	}
	if query.Get("hide_border") == "true" {
		config.HideBorder = true
	}
	if query.Get("hide_rank") == "true" {
		config.HideRank = true
	}
	if query.Get("show_icons") == "true" {
		config.ShowIcons = true
	}

	// Sizing
	if cardWidth := query.Get("card_width"); cardWidth != "" {
		if width, err := strconv.Atoi(cardWidth); err == nil && width > 0 {
			config.CardWidth = width
		}
	}

	// Content
	if customTitle := query.Get("custom_title"); customTitle != "" {
		config.CustomTitle = customTitle
	}

	// Hide/show parameters (comma-separated)
	if hide := query.Get("hide"); hide != "" {
		config.Hide = strings.Split(hide, ",")
	}
	if show := query.Get("show"); show != "" {
		config.Show = strings.Split(show, ",")
	}

	// Advanced
	if locale := query.Get("locale"); locale != "" {
		config.Locale = locale
	}
	if query.Get("disable_animations") == "true" {
		config.DisableAnimations = true
	}
	if borderRadius := query.Get("border_radius"); borderRadius != "" {
		if radius, err := strconv.Atoi(borderRadius); err == nil && radius >= 0 {
			config.BorderRadius = radius
		}
	}

	return config
}

// getGitHubClient creates a GitHub client with authentication
func getGitHubClient() *github.Client {
	if githubToken == "" {
		log.Println("Warning: No GITHUB_TOKEN found, using unauthenticated requests (rate limited)")
		return github.NewClient(nil)
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: githubToken,
	})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	return github.NewClient(oauthClient)
}

// getGraphQLClient creates a GitHub GraphQL client with authentication
func getGraphQLClient() *githubv4.Client {
	if githubToken == "" {
		log.Println("Error: No GITHUB_TOKEN found. GraphQL requires authentication.")
		return nil
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	return githubv4.NewClient(httpClient)
}

// fetchUserStats fetches comprehensive user statistics using GraphQL
func fetchUserStats(ctx context.Context, username string) (*UserStats, error) {
	client := getGraphQLClient()
	if client == nil {
		return nil, fmt.Errorf("GitHub token required for GraphQL API")
	}

	var query struct {
		User struct {
			Name      githubv4.String
			Login     githubv4.String
			AvatarURL githubv4.URI

			// Stats
			PublicRepositories struct {
				TotalCount githubv4.Int
			} `graphql:"public_repos: repositories(first: 1, ownerAffiliations: OWNER, privacy: PUBLIC)"`

			Followers struct {
				TotalCount githubv4.Int
			}
			Following struct {
				TotalCount githubv4.Int
			}

			// Total Stars (iterating all repos is expensive, using a lighter query for total stars if possible,
			// but for now let's use the standard approach of fetching top repos or using Repositories connection)
			// Efficient way to get stars:
			Repositories struct {
				Nodes []struct {
					StargazerCount githubv4.Int
				}
			} `graphql:"all_repos: repositories(first: 100, ownerAffiliations: OWNER, orderBy: {field: STARGAZERS, direction: DESC})"`

			// Contributions
			ContributionsCollection struct {
				TotalCommitContributions      githubv4.Int
				TotalPullRequestContributions githubv4.Int
				TotalIssueContributions       githubv4.Int
				RestrictedContributionsCount  githubv4.Int
			} `graphql:"contributionsCollection(from: $from, to: $to)"`
		} `graphql:"user(login: $username)"`
	}

	// Calculate dates
	now := time.Now()
	currentYear := now.Year()
	startOfLastYear := time.Date(currentYear-1, 1, 1, 0, 0, 0, 0, time.UTC)
	endOfLastYear := time.Date(currentYear-1, 12, 31, 23, 59, 59, 0, time.UTC)

	// Fetch Last Year (2025)
	varsLastYear := map[string]interface{}{
		"username": githubv4.String(username),
		"from":     githubv4.DateTime{Time: startOfLastYear},
		"to":       githubv4.DateTime{Time: endOfLastYear},
	}
	if err := client.Query(ctx, &query, varsLastYear); err != nil {
		return nil, fmt.Errorf("GraphQL query (last year) failed: %v", err)
	}

	// Store last year's counts
	commits := int(query.User.ContributionsCollection.TotalCommitContributions)
	prs := int(query.User.ContributionsCollection.TotalPullRequestContributions)
	issues := int(query.User.ContributionsCollection.TotalIssueContributions)

	// Fetch This Year (2026)
	// We need a separate struct or reset/sum because query struct will be overwritten or we need to query again.
	// Re-using the same struct is fine, the fields will be updated.
	varsThisYear := map[string]interface{}{
		"username": githubv4.String(username),
		"from":     githubv4.DateTime{Time: time.Date(currentYear, 1, 1, 0, 0, 0, 0, time.UTC)},
		"to":       githubv4.DateTime{Time: now},
	}
	if err := client.Query(ctx, &query, varsThisYear); err != nil {
		return nil, fmt.Errorf("GraphQL query (this year) failed: %v", err)
	}

	// Add this year's counts
	commits += int(query.User.ContributionsCollection.TotalCommitContributions)
	prs += int(query.User.ContributionsCollection.TotalPullRequestContributions)
	issues += int(query.User.ContributionsCollection.TotalIssueContributions)

	// Calculate total stars
	var totalStars int
	for _, repo := range query.User.Repositories.Nodes {
		totalStars += int(repo.StargazerCount)
	}

	// Calculate rank (standardized logic now that we have data)
	// Optionally we can include private contributions if we had the scope, but "Restricted" is usually private
	// For public card, usually only public counts matter, but user asked for "Last Year" which usually implies broader activity.
	// GitHub profile shows public + private if you are the user, but public api shows public.
	// We will use public contributions returned by the API.

	stats := &UserStats{
		Name:            string(query.User.Name),
		Login:           string(query.User.Login),
		PublicRepos:     int(query.User.PublicRepositories.TotalCount),
		Followers:       int(query.User.Followers.TotalCount),
		Following:       int(query.User.Following.TotalCount),
		TotalStars:      totalStars,
		TotalCommits:    commits,
		TotalPRs:        prs,
		TotalIssues:     issues,
		ProfileImageURL: query.User.AvatarURL.String(),
	}

	// Fallback for Name if empty
	if stats.Name == "" {
		stats.Name = stats.Login
	}

	return stats, nil
}

// fetchTopRepos fetches user's top repositories
func fetchTopRepos(ctx context.Context, username string, limit int) ([]*Repo, error) {
	client := getGitHubClient()

	repos, _, err := client.Repositories.List(ctx, username, &github.RepositoryListOptions{
		Type:        "owner",
		Sort:        "stargazers",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: limit},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get repos: %w", err)
	}

	var topRepos []*Repo
	for _, repo := range repos {
		topRepos = append(topRepos, &Repo{
			Name:        repo.GetName(),
			Description: repo.GetDescription(),
			Stars:       int(repo.GetStargazersCount()),
			Language:    repo.GetLanguage(),
			HTMLURL:     repo.GetHTMLURL(),
		})
	}

	return topRepos, nil
}

// fetchUserLanguages fetches user's programming language statistics
func fetchUserLanguages(ctx context.Context, username string) (*UserLanguages, error) {
	client := getGitHubClient()

	// Get all user repositories
	repos, _, err := client.Repositories.List(ctx, username, &github.RepositoryListOptions{
		Type:        "owner",
		ListOptions: github.ListOptions{PerPage: 100},
		Affiliation: "owner,collaborator,organization_member",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get repos: %w", err)
	}

	languageBytes := make(map[string]int)
	var totalBytes int

	// Get language statistics for each repo
	for _, repo := range repos {
		languages, _, err := client.Repositories.ListLanguages(ctx, username, repo.GetName())
		if err != nil {
			continue
		}

		for lang, bytes := range languages {
			languageBytes[lang] += int(bytes)
			totalBytes += int(bytes)
		}
	}

	// Calculate percentages
	percentages := make(map[string]float64)
	for lang, bytes := range languageBytes {
		if totalBytes > 0 {
			percentages[lang] = float64(bytes) / float64(totalBytes) * 100
		}
	}

	return &UserLanguages{
		Login:               username,
		Languages:           languageBytes,
		TotalBytes:          totalBytes,
		LanguagePercentages: percentages,
	}, nil
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// statsHandler handles requests for user statistics
func statsHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username parameter is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := fetchUserStats(ctx, username)
	if err != nil {
		log.Printf("Error fetching stats for %s: %v", username, err)
		http.Error(w, fmt.Sprintf("Failed to fetch stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate SVG card
	svg := generateStatsCard(stats)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s", cacheSeconds))

	fmt.Fprint(w, svg)
}

// topReposHandler handles requests for top repositories
func topReposHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username parameter is required", http.StatusBadRequest)
		return
	}

	limit := 6 // default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 20 {
			limit = l
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repos, err := fetchTopRepos(ctx, username, limit)
	if err != nil {
		log.Printf("Error fetching repos for %s: %v", username, err)
		http.Error(w, fmt.Sprintf("Failed to fetch repos: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate SVG card
	svg := generateReposCard(username, repos)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s", cacheSeconds))

	fmt.Fprint(w, svg)
}

// languagesHandler handles requests for language statistics
func languagesHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username parameter is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	languages, err := fetchUserLanguages(ctx, username)
	if err != nil {
		log.Printf("Error fetching languages for %s: %v", username, err)
		http.Error(w, fmt.Sprintf("Failed to fetch languages: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate SVG card
	svg := generateLanguagesCard(languages)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s", cacheSeconds))

	fmt.Fprint(w, svg)
}

// generateStatsCard creates an SVG card with user statistics
func generateStatsCard(stats *UserStats) string {
	// Prepare template data
	data := TemplateData{
		CardWidth:          500,
		CardHeight:         300,
		CardWidthMinus4:    496,
		CardHeightMinus4:   296,
		BorderRadius:       12,
		BorderRadiusMinus2: 10,
		BorderWidth:        2,
		Padding:            25,
		IconOffset:         32,
		ColSpacing:         130,
		ColSpacingMult:     260,
		ColSpacingTriple:   390,

		UserName:     stats.Name,
		Username:     stats.Login,
		Name:         stats.Name,
		CustomTitle:  "GitHub Stats",
		PublicRepos:  stats.PublicRepos,
		Followers:    stats.Followers,
		Following:    stats.Following,
		TotalStars:   stats.TotalStars,
		TotalCommits: stats.TotalCommits,
		TotalPRs:     stats.TotalPRs,
		TotalIssues:  stats.TotalIssues,

		ProgressBarWidth:     140,
		CommitsProgressWidth: calculateProgressWidth(stats.TotalCommits),
		PRsProgressWidth:     calculateProgressWidth(stats.TotalPRs),
		IssuesProgressWidth:  calculateProgressWidth(stats.TotalIssues),
		StatsYStart:          80,
		ContribYStart:        175,

		TitleColor:        "#f0f6fc",
		SubtitleColor:     "#8b949e",
		TextColor:         "#58a6ff",
		SecondaryText:     "#8b949e",
		IconColor:         "#58a6ff",
		StarColor:         "#fbbf24",
		AccentColor:       "#58a6ff",
		AccentColorAlt:    "#3fb950",
		SectionTitleColor: "#f0f6fc",
		FooterColor:       "#8b949e",
		BorderColor:       "#21262d",
		ProgressBg:        "#161b22",
		RankColor:         "#f0f6fc",
		RankBg:            "#161b22",
		RankBorder:        "#21262d",
		ShadowColor:       "#000000",

		BGColor:       "#0d1117",
		CardBg:        "#161b22",
		CardBgAlt:     "#0d1117",
		GradientStart: "#1a1b23",
		GradientEnd:   "#0d1117",

		FontFamily:   "Segoe UI, Arial, sans-serif",
		TitleSize:    20,
		SubtitleSize: 14,

		ShowIcons:         true,
		HideBorder:        false,
		HideTitle:         false,
		HideRepo:          false,
		ShowRank:          false,
		CustomTitleShow:   false,
		ShowFooter:        false,
		DisableAnimations: false,

		HideFollowers:  false,
		HideFollowing:  false,
		HideStars:      false,
		HideCommits:    false,
		HidePRs:        false,
		HideIssues:     false,
		ShowAdditional: false,
		ShowIconsCond:  true,

		RankX:         385,
		Rank:          calculateRank(stats),
		RankY:         15,
		GeneratedDate: time.Now().Format("2006-01-02"),
		FooterText:    "ZoneTwelve Stats",
	}

	// Render using template
	result, err := renderSVGTemplate("stats_card.svg", data)
	if err != nil {
		log.Printf("Template rendering failed: %v", err)
		// Fallback to basic SVG if template fails
		return fmt.Sprintf(`<svg width="450" height="195" xmlns="http://www.w3.org/2000/svg"><text x="10" y="20" fill="red">Template Error: %v</text></svg>`, err)
	}

	return result
}

// generateReposCard creates an SVG card with top repositories
func generateReposCard(username string, repos []*Repo) string {
	cardHeight := 100 + len(repos)*35
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg width="450" height="%d" viewBox="0 0 450 %d" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="title desc">
  <title>Top Repositories for %s</title>
  <desc>User's most starred repositories</desc>

  <!-- Background -->
  <defs>
    <linearGradient id="repoBgGradient" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
      <stop offset="0%%" style="stop-color:#1a1b23;stop-opacity:1" />
      <stop offset="100%%" style="stop-color:#0d1117;stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect width="450" height="%d" fill="url(#repoBgGradient)" rx="12"/>

  <!-- Header -->
  <text x="25" y="35" fill="#f0f6fc" font-family="Segoe UI, Arial, sans-serif" font-size="20" font-weight="600">@%s</text>
  <text x="25" y="55" fill="#8b949e" font-family="Segoe UI, Arial, sans-serif" font-size="14">Top Repositories</text>

`, cardHeight, cardHeight, username, cardHeight, username)

	// Add repositories
	for i, repo := range repos {
		yPos := 85 + i*35
		starsText := ""
		if repo.Stars > 0 {
			starsText = fmt.Sprintf(" ★ %d", repo.Stars)
		}

		svg += fmt.Sprintf(`
  <!-- Repo %d -->
  <rect x="25" y="%d" width="400" height="30" fill="#161b22" rx="6"/>
  <text x="35" y="%d" fill="#58a6ff" font-family="Segoe UI, Arial, sans-serif" font-size="14" font-weight="600">%s%s</text>
`, i+1, yPos, yPos+20, repo.Name, starsText)
	}

	svg += "</svg>"
	return svg
}

// generateLanguagesCard creates an SVG card with language statistics
func generateLanguagesCard(languages *UserLanguages) string {
	cardHeight := 200
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg width="450" height="%d" viewBox="0 0 450 %d" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="title desc">
  <title>Top Languages for %s</title>
  <desc>Programming languages used by the user</desc>

  <!-- Background -->
  <defs>
    <linearGradient id="langBgGradient" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
      <stop offset="0%%" style="stop-color:#1a1b23;stop-opacity:1" />
      <stop offset="100%%" style="stop-color:#0d1117;stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect width="450" height="%d" fill="url(#langBgGradient)" rx="12"/>

  <!-- Header -->
  <text x="25" y="35" fill="#f0f6fc" font-family="Segoe UI, Arial, sans-serif" font-size="20" font-weight="600">@%s</text>
  <text x="25" y="55" fill="#8b949e" font-family="Segoe UI, Arial, sans-serif" font-size="14">Top Languages</text>

`, cardHeight, cardHeight, languages.Login, cardHeight, languages.Login)

	// Add top 6 languages
	langCount := 0
	yPos := 85

	// Sort languages by percentage
	type langWithPercent struct {
		Name    string
		Percent float64
		Bytes   int
	}

	var langs []langWithPercent
	for lang, bytes := range languages.Languages {
		if langCount < 6 && bytes > 0 {
			percent := languages.LanguagePercentages[lang]
			langs = append(langs, langWithPercent{Name: lang, Percent: percent, Bytes: bytes})
			langCount++
		}
	}

	// Add language bars
	for _, lang := range langs {
		width := int(lang.Percent * 3.5) // Scale to fit card width

		svg += fmt.Sprintf(`
  <!-- %s -->
  <rect x="25" y="%d" width="400" height="20" fill="#161b22" rx="4"/>
  <rect x="25" y="%d" width="%d" height="20" fill="%s" rx="4"/>
  <text x="35" y="%d" fill="#f0f6fc" font-family="Segoe UI, Arial, sans-serif" font-size="12">%s (%.1f%%)</text>
`, lang.Name, yPos, yPos, width, getLanguageColor(lang.Name), yPos+14, lang.Name, lang.Percent)

		yPos += 25
	}

	svg += "</svg>"
	return svg
}

// getLanguageColor returns a color for the programming language
func getLanguageColor(lang string) string {
	colors := map[string]string{
		"JavaScript": "#f1e05a",
		"TypeScript": "#3178c6",
		"Python":     "#3572A5",
		"Go":         "#00ADD8",
		"Rust":       "#dea584",
		"Java":       "#b07219",
		"C++":        "#f34b7d",
		"C":          "#555555",
		"C#":         "#178600",
		"PHP":        "#4F5D95",
		"Ruby":       "#701516",
		"Swift":      "#ffac45",
		"Kotlin":     "#A97BFF",
		"Dart":       "#0175C2",
		"Shell":      "#89e051",
		"HTML":       "#e34c26",
		"CSS":        "#1572B6",
		"Vue":        "#4FC08D",
		"React":      "#61DAFB",
	}

	if color, exists := colors[lang]; exists {
		return color
	}
	return "#58a6ff" // Default blue
}

// calculateProgressWidth calculates progress bar width based on commits
func calculateProgressWidth(commits int) int {
	if commits == 0 {
		return 10 // minimum width
	}
	// Scale commits to fit in 140px width (max)
	maxCommits := 1000
	if commits > maxCommits {
		return 140
	}
	return (commits * 140) / maxCommits
}

// calculateRank calculates the user's rank based on their stats
func calculateRank(stats *UserStats) string {
	score := float64(stats.TotalStars)*2.0 +
		float64(stats.Followers)*0.5 +
		float64(stats.TotalCommits)*0.1 +
		float64(stats.TotalPRs)*0.5 +
		float64(stats.TotalIssues)*0.3

	switch {
	case score >= 2000:
		return "S+"
	case score >= 1000:
		return "S"
	case score >= 500:
		return "A+"
	case score >= 200:
		return "A"
	case score >= 100:
		return "B+"
	case score >= 50:
		return "B"
	default:
		return "C"
	}
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Update configuration from environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		githubToken = token
	}
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	if timeout := os.Getenv("CACHE_SECONDS"); timeout != "" {
		cacheSeconds = timeout
	}

	// Load templates
	log.Println("Loading SVG templates...")
	if err := loadTemplates(); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}
	log.Println("Templates loaded successfully")

	// Setup routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api", statsHandler)
	http.HandleFunc("/api/top-langs", languagesHandler)
	http.HandleFunc("/api/pin", topReposHandler)

	// Add CORS headers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Default handler
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "GitHub Readme Stats API",
			"endpoints": map[string]interface{}{
				"stats":     "/api?username=your_username",
				"languages": "/api/top-langs?username=your_username",
				"repos":     "/api/pin?username=your_username",
				"health":    "/health",
			},
			"example": "Try: /api?username=ZoneTwelve",
		})
	})

	// Start server
	addr := ":" + port
	fmt.Printf("🚀 GitHub Readme Stats Server starting on port %s\n", port)
	fmt.Printf("📊 Endpoints:\n")
	fmt.Printf("   Stats:     http://localhost%s/api?username=ZoneTwelve\n", addr)
	fmt.Printf("   Languages: http://localhost%s/api/top-langs?username=ZoneTwelve\n", addr)
	fmt.Printf("   Repos:     http://localhost%s/api/pin?username=ZoneTwelve&limit=6\n", addr)
	fmt.Printf("   Health:    http://localhost%s/health\n", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
