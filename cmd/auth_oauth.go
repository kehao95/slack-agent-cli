package cmd

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/kehao95/slack-agent-cli/internal/policy"
	"github.com/spf13/cobra"
)

const (
	defaultOAuthUserScopes = "identify,channels:read,channels:history,channels:write,groups:read,groups:history,groups:write,im:read,im:history,im:write,mpim:read,mpim:history,mpim:write,chat:write,users:read,users:read.email,users.profile:read,users.profile:write,users:write,dnd:write,usergroups:read,usergroups:write,search:read,reactions:read,reactions:write,pins:read,pins:write,files:read,files:write,lists:read,lists:write,canvases:read,canvases:write,bookmarks:read,bookmarks:write,dnd:read,emoji:read"
	defaultOAuthBotScopes  = "channels:manage,channels:read,channels:history,groups:read,groups:history,groups:write,im:read,im:history,im:write,mpim:read,mpim:history,mpim:write,chat:write,users:read,users:read.email,users.profile:read,users:write,usergroups:read,usergroups:write,reactions:read,reactions:write,pins:read,pins:write,files:read,files:write,lists:read,lists:write,canvases:read,canvases:write,bookmarks:read,bookmarks:write,dnd:read,emoji:read"
)

var (
	oauthPort         int
	oauthClientID     string
	oauthClientSecret string
	oauthRedirectURI  string
	oauthScopes       string
	oauthBotScopes    string
	oauthSaveConfig   bool
)

var authOAuthCmd = &cobra.Command{
	Use:   "oauth",
	Short: "Start OAuth callback server for token exchange",
	Long: `Start a local HTTP server to handle Slack OAuth callback.

This server receives the authorization code from Slack and exchanges it
for an access token. Expose the server publicly and configure your Slack
app's redirect URI to point to the /callback endpoint. The flow validates a
single-use state value and saves credentials without displaying raw tokens.`,
	Example: `  # Start OAuth server on default port
  slk auth oauth --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET

  # With custom port and redirect URI
  slk auth oauth --port 9000 --client-id ID --client-secret SECRET --redirect-uri https://example.com/callback

  # Exchange credentials but intentionally discard them
  slk auth oauth --client-id ID --client-secret SECRET --save=false`,
	RunE: runAuthOAuth,
}

func init() {
	authCmd.AddCommand(authOAuthCmd)

	authOAuthCmd.Flags().IntVarP(&oauthPort, "port", "p", 8089, "Port to listen on")
	authOAuthCmd.Flags().StringVar(&oauthClientID, "client-id", "", "Slack app client ID (or SLACK_CLIENT_ID env)")
	authOAuthCmd.Flags().StringVar(&oauthClientSecret, "client-secret", "", "Slack app client secret (or SLACK_CLIENT_SECRET env)")
	authOAuthCmd.Flags().StringVar(&oauthRedirectURI, "redirect-uri", "", "OAuth redirect URI (optional, for token exchange)")
	authOAuthCmd.Flags().StringVar(&oauthScopes, "scopes", defaultOAuthUserScopes, "Comma-separated OAuth user scopes to request")
	authOAuthCmd.Flags().StringVar(&oauthBotScopes, "bot-scopes", defaultOAuthBotScopes, "Comma-separated OAuth bot scopes to request; empty disables bot-token installation")
	authOAuthCmd.Flags().BoolVar(&oauthSaveConfig, "save", true, "Save user and bot tokens to config (use --save=false only to discard them)")
}

// OAuthTokenResponse represents Slack's oauth.v2.access response
type OAuthTokenResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	BotUserID   string `json:"bot_user_id,omitempty"`
	AppID       string `json:"app_id,omitempty"`
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team,omitempty"`
	AuthedUser struct {
		ID          string `json:"id"`
		Scope       string `json:"scope"`
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	} `json:"authed_user,omitempty"`
}

type oauthCallbackOptions struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	State        *oneTimeOAuthState
	Save         bool
	ConfigPath   string
	Exchange     func(code, clientID, clientSecret, redirectURI string) (*OAuthTokenResponse, error)
	Log          io.Writer
}

type oneTimeOAuthState struct {
	value    string
	mu       sync.Mutex
	consumed bool
}

func newOAuthState() (*oneTimeOAuthState, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate OAuth state: %w", err)
	}
	return &oneTimeOAuthState{value: base64.RawURLEncoding.EncodeToString(buf)}, nil
}

func (s *oneTimeOAuthState) validateAndConsume(candidate string) bool {
	if s == nil || candidate == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consumed || subtle.ConstantTimeCompare([]byte(candidate), []byte(s.value)) != 1 {
		return false
	}
	s.consumed = true
	return true
}

func runAuthOAuth(cmd *cobra.Command, args []string) error {
	if err := policy.CheckMethod("oauth.v2.access"); err != nil {
		return err
	}
	// Get credentials from flags or environment
	clientID := oauthClientID
	if clientID == "" {
		clientID = os.Getenv("SLACK_CLIENT_ID")
	}
	clientSecret := oauthClientSecret
	if clientSecret == "" {
		clientSecret = os.Getenv("SLACK_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("client-id and client-secret are required (use flags or SLACK_CLIENT_ID/SLACK_CLIENT_SECRET env vars)")
	}
	state, err := newOAuthState()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// OAuth callback endpoint
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthCallback(w, r, oauthCallbackOptions{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  oauthRedirectURI,
			State:        state,
			Save:         oauthSaveConfig,
			ConfigPath:   cfgFile,
			Exchange:     exchangeCodeForToken,
			Log:          os.Stderr,
		})
	})

	// Root endpoint - shows instructions
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		authURL := buildAuthURL(clientID, oauthRedirectURI, oauthScopes, oauthBotScopes, state.value)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Slack OAuth</title></head>
<body>
<h1>Slack OAuth Server</h1>
<p>Server is running. Configure your Slack app redirect URI to point to <code>/callback</code> on this server.</p>
<p><a href="%s">Click here to authorize</a> (update redirect_uri in link if using cloudflared)</p>
<h2>Endpoints:</h2>
<ul>
<li><code>GET /</code> - This page</li>
<li><code>GET /callback?code=XXX</code> - OAuth callback (exchanges code for token)</li>
<li><code>GET /health</code> - Health check</li>
</ul>
</body>
</html>`, authURL)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", oauthPort),
		Handler: mux,
	}

	// Handle graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		fmt.Fprintln(os.Stderr, "\nShutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "OAuth callback server listening on http://localhost:%d\n", oauthPort)
	fmt.Fprintf(os.Stderr, "Endpoints:\n")
	fmt.Fprintf(os.Stderr, "  GET /          - Instructions and auth link\n")
	fmt.Fprintf(os.Stderr, "  GET /callback  - OAuth callback (receives code, exchanges for token)\n")
	fmt.Fprintf(os.Stderr, "  GET /health    - Health check\n")
	fmt.Fprintf(os.Stderr, "\nPress Ctrl+C to stop\n\n")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, options oauthCallbackOptions) {
	log := options.Log
	if log == nil {
		log = os.Stderr
	}
	w.Header().Set("Content-Type", "application/json")
	if !options.State.validateAndConsume(r.URL.Query().Get("state")) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "invalid_state",
		})
		fmt.Fprintln(log, "OAuth callback rejected: invalid or reused state")
		return
	}

	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"error":       errorParam,
			"description": errorDesc,
		})
		fmt.Fprintf(log, "OAuth error: %s - %s\n", errorParam, errorDesc)
		return
	}

	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "missing_code",
		})
		return
	}

	fmt.Fprintln(log, "Received authorization code, exchanging for token...")

	// Exchange code for token
	exchange := options.Exchange
	if exchange == nil {
		exchange = exchangeCodeForToken
	}
	tokenResp, err := exchange(code, options.ClientID, options.ClientSecret, options.RedirectURI)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "token_exchange_failed",
		})
		fmt.Fprintf(log, "Token exchange error: %v\n", err)
		return
	}
	if tokenResp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "token_exchange_failed"})
		fmt.Fprintln(log, "Token exchange returned an empty response")
		return
	}

	if !tokenResp.OK {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": tokenResp.Error,
		})
		fmt.Fprintf(log, "Slack API error: %s\n", tokenResp.Error)
		return
	}

	// Save to config if requested
	saved := false
	if options.Save {
		if tokenResp.AuthedUser.AccessToken == "" && tokenResp.AccessToken == "" {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "missing_access_token"})
			fmt.Fprintln(log, "OAuth token exchange succeeded without an access token")
			return
		}
		if err := saveOAuthTokensToConfig(options.ConfigPath, tokenResp.AuthedUser.AccessToken, tokenResp.AccessToken); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "save_failed"})
			fmt.Fprintf(log, "Failed to save OAuth credentials: %v\n", err)
			return
		}
		saved = true
		fmt.Fprintln(log, "OAuth credentials saved to config file")
	}

	// Return metadata only. Access and refresh tokens must never reach the browser or logs.
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"team_id":     tokenResp.Team.ID,
		"team_name":   tokenResp.Team.Name,
		"user_id":     tokenResp.AuthedUser.ID,
		"bot_user_id": tokenResp.BotUserID,
		"saved":       saved,
	})

	// Print non-sensitive metadata to stderr for operator visibility.
	fmt.Fprintln(log, "\n=== OAuth Success ===")
	fmt.Fprintf(log, "Team: %s (%s)\n", tokenResp.Team.Name, tokenResp.Team.ID)
	if tokenResp.AuthedUser.ID != "" {
		fmt.Fprintf(log, "User ID: %s\n", tokenResp.AuthedUser.ID)
		fmt.Fprintf(log, "User Scopes: %s\n", tokenResp.AuthedUser.Scope)
	}
	if tokenResp.BotUserID != "" {
		fmt.Fprintf(log, "Bot User ID: %s\n", tokenResp.BotUserID)
		fmt.Fprintf(log, "Bot Scopes: %s\n", tokenResp.Scope)
	}
	fmt.Fprintln(log, "=====================")
}

func exchangeCodeForToken(code, clientID, clientSecret, redirectURI string) (*OAuthTokenResponse, error) {
	if err := policy.CheckMethod("oauth.v2.access"); err != nil {
		return nil, err
	}
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	if redirectURI != "" {
		data.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/oauth.v2.access", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &tokenResp, nil
}

func buildAuthURL(clientID, redirectURI, userScopes, botScopes, state string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("user_scope", userScopes)
	if strings.TrimSpace(botScopes) != "" {
		params.Set("scope", botScopes)
	}
	params.Set("state", state)
	if redirectURI != "" {
		params.Set("redirect_uri", redirectURI)
	}
	return "https://slack.com/oauth/v2/authorize?" + params.Encode()
}

func saveOAuthTokensToConfig(path, userToken, botToken string) error {
	cfg, resolvedPath, err := config.Load(path)
	if err != nil {
		return err
	}
	if userToken != "" {
		cfg.UserToken = userToken
		cfg.Role = config.RoleUser
	}
	if botToken != "" {
		cfg.BotToken = botToken
		if userToken == "" {
			cfg.Role = config.RoleBot
		}
	}

	_, err = config.Save(resolvedPath, cfg)
	return err
}
