package payloadschema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateNewsItemPayload_Valid(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"hackernews",
		"source_item_id":"12345",
		"title":"Model release",
		"source_metadata":{
			"collection":"ai_news",
			"job_name":"openclaw-ai-daily",
			"job_run_id":"run_2026_02_14_001",
			"scraped_at":"2026-02-14T10:00:00Z",
			"score":42
		},
		"canonical_url":"https://example.com/story/12345",
		"published_at":"2026-02-13T14:00:00Z",
		"authors":["Alice","Bob"],
		"tags":["ai","release"]
	}`)

	item, err := ValidateNewsItemPayload(payload)
	if err != nil {
		t.Fatalf("expected payload to be valid, got error: %v", err)
	}

	if item.Source != "hackernews" {
		t.Fatalf("expected source=hackernews, got %q", item.Source)
	}
	if item.PayloadVersion != "v1" {
		t.Fatalf("expected payload_version=v1, got %q", item.PayloadVersion)
	}
}

func TestValidateNewsItemPayload_MissingRequired(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"reddit",
		"title":"Missing source item id",
		"source_metadata":{
			"collection":"world_news",
			"job_name":"openclaw-world-daily",
			"job_run_id":"run_2026_02_14_002",
			"scraped_at":"2026-02-14T10:00:00Z"
		}
	}`)

	_, err := ValidateNewsItemPayload(payload)
	if err == nil {
		t.Fatalf("expected validation to fail for missing source_item_id")
	}
}

func TestValidateNewsItemPayload_WhitespaceTitle(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"reddit",
		"source_item_id":"abc",
		"title":"   ",
		"source_metadata":{
			"collection":"world_news",
			"job_name":"openclaw-world-daily",
			"job_run_id":"run_2026_02_14_003",
			"scraped_at":"2026-02-14T10:00:00Z"
		}
	}`)

	_, err := ValidateNewsItemPayload(payload)
	if err == nil {
		t.Fatalf("expected validation to fail for whitespace-only title")
	}
	if !strings.Contains(err.Error(), "title must not be empty") {
		t.Fatalf("expected title semantic error, got: %v", err)
	}
}

func TestValidateNewsItemPayload_InvalidPublishedAt(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"rss",
		"source_item_id":"id-1",
		"title":"Bad date",
		"published_at":"not-a-timestamp",
		"source_metadata":{
			"collection":"china_news",
			"job_name":"openclaw-china-daily",
			"job_run_id":"run_2026_02_14_004",
			"scraped_at":"2026-02-14T10:00:00Z"
		}
	}`)

	_, err := ValidateNewsItemPayload(payload)
	if err == nil {
		t.Fatalf("expected validation to fail for invalid published_at")
	}
}

func TestValidateNewsItemPayload_WithCollectionMetadata(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"rss",
		"source_item_id":"id-collection-1",
		"title":"Tagged collection payload",
		"source_metadata":{
			"collection":"ai_news",
			"job_name":"openclaw-ai-daily",
			"job_run_id":"run_2026_02_14_001",
			"scraped_at":"2026-02-14T10:00:00Z"
		}
	}`)

	_, err := ValidateNewsItemPayload(payload)
	if err != nil {
		t.Fatalf("expected payload with collection metadata to be valid, got error: %v", err)
	}
}

func TestValidateNewsItemPayload_CollectionMustBeString(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"rss",
		"source_item_id":"id-collection-2",
		"title":"Bad collection type",
		"source_metadata":{
			"collection":123,
			"job_name":"openclaw-ai-daily",
			"job_run_id":"run_2026_02_14_005",
			"scraped_at":"2026-02-14T10:00:00Z"
		}
	}`)

	_, err := ValidateNewsItemPayload(payload)
	if err == nil {
		t.Fatalf("expected validation to fail when source_metadata.collection is not a string")
	}
}

func TestValidateNewsItemPayload_MetadataInnerKeysRequired(t *testing.T) {
	payload := json.RawMessage(`{
		"payload_version":"v1",
		"source":"rss",
		"source_item_id":"id-collection-3",
		"title":"Missing metadata keys",
		"source_metadata":{
			"collection":"ai_news"
		}
	}`)

	_, err := ValidateNewsItemPayload(payload)
	if err == nil {
		t.Fatalf("expected validation to fail when source_metadata required keys are missing")
	}
}
