package pipeline

import "testing"

func TestNormalizeEmbeddingEndpoint(t *testing.T) {
	t.Parallel()

	if got := normalizeEmbeddingEndpoint("http://127.0.0.1:8844"); got != "http://127.0.0.1:8844/embed" {
		t.Fatalf("unexpected endpoint normalization: %q", got)
	}
	if got := normalizeEmbeddingEndpoint("http://127.0.0.1:8844/v1/embeddings"); got != "http://127.0.0.1:8844/v1/embeddings" {
		t.Fatalf("unexpected endpoint normalization for explicit path: %q", got)
	}
}

func TestToVectorLiteralDimensionValidation(t *testing.T) {
	t.Parallel()

	_, err := toVectorLiteral([]float64{0.1, 0.2})
	if err == nil {
		t.Fatalf("expected dimension validation error for short vector")
	}
}

func TestSemanticThresholdHelpers(t *testing.T) {
	t.Parallel()

	if !shouldAutoMergeSemantic(0.97, 0.05) {
		t.Fatalf("expected override cosine threshold to auto-merge")
	}
	if !shouldAutoMergeSemantic(0.94, 0.35) {
		t.Fatalf("expected cosine+title threshold to auto-merge")
	}
	if shouldAutoMergeSemantic(0.92, 0.50) {
		t.Fatalf("did not expect low cosine to auto-merge")
	}

	composite := semanticCompositeScore(0.9, 0.4, 1.0)
	if composite <= 0 || composite > 1 {
		t.Fatalf("expected composite score in (0,1], got %f", composite)
	}
}
