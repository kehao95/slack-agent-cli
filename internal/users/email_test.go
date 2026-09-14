package users

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

func TestSuccessfulUserLookupsReportEmailAvailability(t *testing.T) {
	for _, email := range []string{"", "alice@example.com"} {
		for _, operation := range []string{"info", "list", "lookup", "profile"} {
			t.Run(operation+"/"+email, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					expected := map[string]string{"info": "/users.info", "list": "/users.list", "lookup": "/users.lookupByEmail", "profile": "/users.profile.get"}[operation]
					if r.URL.Path != expected {
						t.Errorf("unexpected endpoint %s", r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					profile := map[string]interface{}{"display_name": "Alice"}
					if email != "" {
						profile["email"] = email
					}
					user := map[string]interface{}{"id": "U123", "name": "alice", "profile": profile}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "user": user, "members": []interface{}{user}, "profile": profile})
				}))
				defer server.Close()
				service := NewService(appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/")))
				var result interface{ Lines() []string }
				var err error
				switch operation {
				case "info":
					result, err = service.GetInfo(context.Background(), "U123")
				case "list":
					result, err = service.List(context.Background(), ListParams{})
				case "lookup":
					result, err = service.LookupByEmail(context.Background(), "alice@example.com")
				case "profile":
					result, err = service.GetProfile(context.Background(), "U123", false)
				}
				if err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("expected one lookup without scope guesses, got %d", calls)
				}
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				var output map[string]interface{}
				if err := json.Unmarshal(data, &output); err != nil {
					t.Fatal(err)
				}
				if output["ok"] != true {
					t.Fatalf("found user was not successful: %s", data)
				}
				entry := output
				if operation == "list" {
					entry = output["users"].([]interface{})[0].(map[string]interface{})
				} else if operation != "profile" {
					entry = output["user"].(map[string]interface{})
				}
				if available, exists := entry["email_available"]; !exists || available != (email != "") {
					t.Fatalf("incorrect availability: %s", data)
				}
				human := strings.Join(result.Lines(), "\n")
				if email == "" {
					if strings.Contains(string(data), "alice@example.com") {
						t.Fatalf("fabricated email: %s", data)
					}
					if !strings.Contains(human, "unavailable") || !strings.Contains(human, "users:read.email") || !strings.Contains(human, "other reasons") {
						t.Fatalf("missing honest availability explanation: %s", human)
					}
				} else if strings.Contains(human, "unavailable") {
					t.Fatalf("available email labeled unavailable: %s", human)
				}
			})
		}
	}
}

func TestMissingUserRemainsLookupFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":false,"error":"user_not_found"}`)
	}))
	defer server.Close()
	service := NewService(appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/")))
	for _, operation := range []string{"info", "lookup", "profile"} {
		t.Run(operation, func(t *testing.T) {
			var err error
			switch operation {
			case "info":
				result, resultErr := service.GetInfo(context.Background(), "U404")
				err = resultErr
				if result != nil {
					t.Fatalf("unexpected successful result: %+v", result)
				}
			case "lookup":
				result, resultErr := service.LookupByEmail(context.Background(), "missing@example.com")
				err = resultErr
				if result != nil {
					t.Fatalf("unexpected successful result: %+v", result)
				}
			case "profile":
				result, resultErr := service.GetProfile(context.Background(), "U404", false)
				err = resultErr
				if result != nil {
					t.Fatalf("unexpected successful result: %+v", result)
				}
			}
			if err == nil || !strings.Contains(err.Error(), "user_not_found") {
				t.Fatalf("expected actual lookup error, got %v", err)
			}
		})
	}
}
