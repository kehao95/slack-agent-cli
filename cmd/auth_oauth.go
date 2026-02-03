package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	oauthPort         int
	oauthClientID     string
	oauthClientSecret string
	oauthRedirectURI  string
	oauthScopes       string
	oauthSaveConfig   bool
)

var authOAuthCmd = &cobra.Command{
	Use:   "oauth",
	Short: "Start OAuth callback server for token exchange",
	Long: `Start a local HTTP server to handle Slack OAuth callback.

This server receives the authorization code from Slack and exchanges it
for an access token. Expose the server publicly and configure your Slack
app's redirect URI to point to the /callback endpoint.`,
	Example: `  # Start OAuth server on default port
  slk auth oauth --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET

  # With custom port and redirect URI
  slk auth oauth --port 9000 --client-id ID --client-secret SECRET --redirect-uri https://example.com/callback

  # Auto-save token to config after successful exchange
  slk auth oauth --client-id ID --client-secret SECRET --save`,
	RunE: runAuthOAuth,
}

func init() {
	authCmd.AddCommand(authOAuthCmd)

	authOAuthCmd.Flags().IntVarP(&oauthPort, "port", "p", 8089, "Port to listen on")
	authOAuthCmd.Flags().StringVar(&oauthClientID, "client-id", "", "Slack app client ID (or SLACK_CLIENT_ID env)")
	authOAuthCmd.Flags().StringVar(&oauthClientSecret, "client-secret", "", "Slack app client secret (or SLACK_CLIENT_SECRET env)")
	authOAuthCmd.Flags().StringVar(&oauthRedirectURI, "redirect-uri", "", "OAuth redirect URI (optional, for token exchange)")
	authOAuthCmd.Flags().StringVar(&oauthScopes, "scopes", "channels:read,channels:history,chat:write,users:read,search:read,reactions:read,reactions:write,pins:read,pins:write,emoji:read", "OAuth scopes to request")
	authOAuthCmd.Flags().BoolVar(&oauthSaveConfig, "save", false, "Save token to config file after successful exchange")
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

func runAuthOAuth(cmd *cobra.Command, args []string) error {
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

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// OAuth callback endpoint
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthCallback(w, r, clientID, clientSecret)
	})

	// Root endpoint - shows instructions
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		authURL := buildAuthURL(clientID, oauthRedirectURI, oauthScopes)
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

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, clientID, clientSecret string) {
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"ok":          "false",
			"error":       errorParam,
			"description": errorDesc,
		})
		fmt.Fprintf(os.Stderr, "OAuth error: %s - %s\n", errorParam, errorDesc)
		return
	}

	if code == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"ok":    "false",
			"error": "missing_code",
		})
		return
	}

	fmt.Fprintf(os.Stderr, "Received authorization code, exchanging for token...\n")

	// Exchange code for token
	tokenResp, err := exchangeCodeForToken(code, clientID, clientSecret, oauthRedirectURI)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"ok":    "false",
			"error": err.Error(),
		})
		fmt.Fprintf(os.Stderr, "Token exchange error: %v\n", err)
		return
	}

	if !tokenResp.OK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenResp)
		fmt.Fprintf(os.Stderr, "Slack API error: %s\n", tokenResp.Error)
		return
	}

	// Determine which token to use (user token preferred)
	token := tokenResp.AuthedUser.AccessToken
	if token == "" {
		token = tokenResp.AccessToken
	}

	// Save to config if requested
	if oauthSaveConfig && token != "" {
		if err := saveTokenToConfig(token); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save token to config: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Token saved to config file\n")
		}
	}

	// Output success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResp)

	// Also print to stderr for easy copying
	fmt.Fprintf(os.Stderr, "\n=== OAuth Success ===\n")
	fmt.Fprintf(os.Stderr, "Team: %s (%s)\n", tokenResp.Team.Name, tokenResp.Team.ID)
	if tokenResp.AuthedUser.ID != "" {
		fmt.Fprintf(os.Stderr, "User ID: %s\n", tokenResp.AuthedUser.ID)
		fmt.Fprintf(os.Stderr, "User Token: %s\n", tokenResp.AuthedUser.AccessToken)
		fmt.Fprintf(os.Stderr, "User Scopes: %s\n", tokenResp.AuthedUser.Scope)
	}
	if tokenResp.AccessToken != "" {
		fmt.Fprintf(os.Stderr, "Bot Token: %s\n", tokenResp.AccessToken)
		fmt.Fprintf(os.Stderr, "Bot Scopes: %s\n", tokenResp.Scope)
	}
	fmt.Fprintf(os.Stderr, "=====================\n")
}

func exchangeCodeForToken(code, clientID, clientSecret, redirectURI string) (*OAuthTokenResponse, error) {
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

func buildAuthURL(clientID, redirectURI, scopes string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("user_scope", scopes)
	if redirectURI != "" {
		params.Set("redirect_uri", redirectURI)
	}
	return "https://slack.com/oauth/v2/authorize?" + params.Encode()
}

func saveTokenToConfig(token string) error {
	cfg, path, err := config.Load("")
	if err != nil {
		// If config doesn't exist, create default
		cfg = config.DefaultConfig()
	}
	cfg.UserToken = token

	_, err = config.Save(path, cfg)
	return err
}
