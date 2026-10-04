package voicast_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	voicast "github.com/shannonllc/voicast-go"
)

func Example() {
	client, err := voicast.NewClient("vk_live_...") // or set VOICAST_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	agent, err := client.Agents.Create(ctx, voicast.AgentCreateParams{
		Name: "Reception",
		Config: voicast.AgentConfig{
			Greeting:    "お電話ありがとうございます。○○歯科です。",
			Persona:     "丁寧で落ち着いた受付",
			Fields:      []voicast.AgentField{{Key: "name", Label: "お名前", Required: true}, {Key: "phone", Label: "電話番号", Type: "phone"}},
			HandoffWhen: []string{"急患"},
			Closing:     "承りました。担当者から折り返しご連絡します。",
			Voice:       "Kore",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	tenant, err := client.Tenants.Create(ctx, voicast.TenantCreateParams{ExternalID: "clinic-123", Name: "○○歯科", AgentID: agent.ID})
	var e *voicast.Error
	switch {
	case errors.As(err, &e) && e.Kind == voicast.KindConflict:
		fmt.Println("already exists:", e.Code)
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Println(tenant.ID, tenant.Secret) // the secret (vts_...) is returned only here
	}
}

func ExampleVerifyWebhook() {
	secret := os.Getenv("VOICAST_WEBHOOK_SECRET") // whsec_...
	http.HandleFunc("/webhooks/voicast", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		event, err := voicast.VerifyWebhook(body, r.Header, secret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, _ := event.ParseData()
		switch d := data.(type) {
		case *voicast.CallEndedData:
			fmt.Println(*d.Outcome, d.Fields, d.Tenant.ExternalID)
		case *voicast.CallSummaryReadyData:
			fmt.Println(d.Summary, d.Category)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func ExampleVerifyToolCall() {
	secret := os.Getenv("VOICAST_TOOLS_SECRET") // whsec_...
	http.HandleFunc("/voicast/tools", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call, err := voicast.VerifyToolCall(body, r.Header, secret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var args struct {
			Date string `json:"date"`
		}
		_ = call.BindArguments(&args)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"slots": []string{args.Date + "T10:00"}, "test": call.Test})
	})
}

func ExampleCallsService_ListAll() {
	client, _ := voicast.NewClient("vk_live_...")
	ctx := context.Background()
	it := client.Calls.ListAll(&voicast.CallListParams{TenantID: "ten_..."})
	for it.Next(ctx) {
		fmt.Println(it.Current().ID)
	}
	if err := it.Err(); err != nil {
		log.Fatal(err)
	}
}
