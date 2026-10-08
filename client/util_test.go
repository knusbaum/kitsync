package client

import (
	"testing"
)

func TestMergeKeylessEmptyExisting(t *testing.T) {
	got := MergeKeyless("", "alpha beta")
	if got != "alpha beta" {
		t.Errorf("MergeKeyless(\"\", \"alpha beta\") = %q, want \"alpha beta\"", got)
	}
}

func TestMergeKeylessEmptyBoth(t *testing.T) {
	got := MergeKeyless("", "")
	if got != "" {
		t.Errorf("MergeKeyless(\"\", \"\") = %q, want \"\"", got)
	}
}

func TestMergeKeylessNewOnly(t *testing.T) {
	got := MergeKeyless("", "new")
	if got != "new" {
		t.Errorf("MergeKeyless(\"\", \"new\") = %q, want \"new\"", got)
	}
}

func TestMergeKeylessDuplicateSkipped(t *testing.T) {
	got := MergeKeyless("alpha", "alpha beta")
	if got != "alpha beta" {
		t.Errorf("MergeKeyless(\"alpha\", \"alpha beta\") = %q, want \"alpha beta\"", got)
	}
}

func TestMergeKeylessAlreadyHasBoth(t *testing.T) {
	got := MergeKeyless("alpha beta", "gamma")
	if got != "alpha beta gamma" {
		t.Errorf("MergeKeyless(\"alpha beta\", \"gamma\") = %q, want \"alpha beta gamma\"", got)
	}
}

func TestMergeKeylessAlreadyHasAll(t *testing.T) {
	got := MergeKeyless("alpha beta", "")
	if got != "alpha beta" {
		t.Errorf("MergeKeyless(\"alpha beta\", \"\") = %q, want \"alpha beta\"", got)
	}
}

func TestMergeKeylessTrimmedSpaces(t *testing.T) {
	got := MergeKeyless("", "  alpha   ")
	if got != "alpha" {
		t.Errorf("MergeKeyless(\"\", \"  alpha   \") = %q, want \"alpha\"", got)
	}
}

func TestRemoveKeylessEmptyExisting(t *testing.T) {
	got := RemoveKeyless("", "alpha")
	if got != "" {
		t.Errorf("RemoveKeyless(\"\", \"alpha\") = %q, want \"\"", got)
	}
}

func TestRemoveKeylessNothingToRemove(t *testing.T) {
	got := RemoveKeyless("alpha beta", "gamma")
	if got != "alpha beta" {
		t.Errorf("RemoveKeyless(\"alpha beta\", \"gamma\") = %q, want \"alpha beta\"", got)
	}
}

func TestRemoveKeylessSingleToRemove(t *testing.T) {
	got := RemoveKeyless("alpha beta", "alpha")
	if got != "beta" {
		t.Errorf("RemoveKeyless(\"alpha beta\", \"alpha\") = %q, want \"beta\"", got)
	}
}

func TestRemoveKeylessAllRemoved(t *testing.T) {
	got := RemoveKeyless("alpha beta", "alpha beta")
	if got != "" {
		t.Errorf("RemoveKeyless(\"alpha beta\", \"alpha beta\") = %q, want \"\"", got)
	}
}

func TestRemoveKeylessMultipleToRemove(t *testing.T) {
	got := RemoveKeyless("a b c d", "b d")
	if got != "a c" {
		t.Errorf("RemoveKeyless(\"a b c d\", \"b d\") = %q, want \"a c\"", got)
	}
}

func TestMergeTagsSimple(t *testing.T) {
	dst := map[string]string{"a": "1"}
	src := map[string]string{"b": "2", "c": "3"}
	MergeTags(dst, src)
	if len(dst) != 3 {
		t.Fatalf("len(MergeTags) = %d, want 3", len(dst))
	}
	if dst["a"] != "1" || dst["b"] != "2" || dst["c"] != "3" {
		t.Errorf("MergeTags result = %v, want map[a:1 b:2 c:3]", dst)
	}
}

func TestMergeTagsOverwrite(t *testing.T) {
	dst := map[string]string{"x": "old"}
	src := map[string]string{"x": "new"}
	MergeTags(dst, src)
	if dst["x"] != "new" {
		t.Errorf("MergeTags overwrote x = %q, want \"new\"", dst["x"])
	}
}

func TestMergeTagsKeyless(t *testing.T) {
	dst := map[string]string{"": "alpha"}
	src := map[string]string{"": "beta gamma"}
	MergeTags(dst, src)
	if dst[""] != "alpha beta gamma" {
		t.Errorf("keyless merge = %q, want \"alpha beta gamma\"", dst[""])
	}
}
