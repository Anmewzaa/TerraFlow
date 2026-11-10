package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/store/storetest"
)

func TestProvidersAreSeededAndRawResourcesBelongToOne(t *testing.T) {
	q, _ := storetest.New(t)
	ctx := context.Background()
	list, err := q.ListProviders(ctx)
	if err != nil || len(list) == 0 || list[0].ID != "azurerm" || len(list[0].Versions) == 0 || list[0].Logo == "" {
		t.Fatalf("the seeded provider: %+v %v", list, err)
	}
	if err := q.CreateProvider(ctx, store.Provider{ID: "azurerm", Name: "x", Source: "a/b"}); !errors.Is(err, store.ErrProviderExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := q.CreateProvider(ctx, store.Provider{ID: "aws", Name: "AWS", Source: "hashicorp/aws", Versions: []string{"~> 5.0"}}); err != nil {
		t.Fatal(err)
	}
	raw := store.RawResource{Name: "bucket", Title: "Bucket", ResourceType: "aws_s3_bucket", DefaultValues: map[string]interface{}{}, Provider: "aws", Category: "storage", MinVersion: "4.0", MaxVersion: "5.9"}
	if err := q.CreateRawResource(ctx, raw); err != nil {
		t.Fatal(err)
	}
	got, _ := q.ListRawResources(ctx)
	if len(got) != 1 || got[0].Provider != "aws" || got[0].Category != "storage" || got[0].MinVersion != "4.0" || got[0].MaxVersion != "5.9" {
		t.Fatalf("raw resource: %+v", got)
	}
	if err := q.DeleteProvider(ctx, "aws"); !errors.Is(err, store.ErrProviderInUse) {
		t.Fatalf("delete in use: %v", err)
	}
	_ = q.DeleteRawResource(ctx, "bucket")
	if err := q.DeleteProvider(ctx, "aws"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
