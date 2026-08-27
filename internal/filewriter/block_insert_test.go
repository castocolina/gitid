package filewriter

import (
	"bytes"
	"strings"
	"testing"
)

func TestInsertBlockAfter_PlacesImmediatelyAfterAnchor(t *testing.T) {
	anchorBegin := BeginPrefix + "baseline-include"
	anchorEnd := EndPrefix + "baseline-include"
	existing := []byte(
		"# header above\n" +
			anchorBegin + "\n" +
			"[include]\n\tpath = ~/.gitconfig.d/00-baseline\n" +
			anchorEnd + "\n" +
			"[includeIf \"gitdir:~/git/work/\"]\n\tpath = ~/.gitconfig.d/work\n",
	)

	out, err := InsertBlockAfter(existing, "baseline-include", "global-git-author", "[user]\n\tname = Pat")
	if err != nil {
		t.Fatalf("InsertBlockAfter: %v", err)
	}
	got := string(out)

	anchorEndLine := anchorEnd + "\n"
	endOff := strings.Index(got, anchorEndLine)
	if endOff < 0 {
		t.Fatalf("anchor end marker missing:\n%s", got)
	}
	insertOff := endOff + len(anchorEndLine)
	newBegin := BeginPrefix + "global-git-author\n"
	if !strings.HasPrefix(got[insertOff:], newBegin) {
		t.Fatalf("new block begin-marker is not immediately after the anchor end-marker line\n got at insert: %q\n want prefix: %q\n full:\n%s",
			got[insertOff:], newBegin, got)
	}

	above := existing[:bytes.Index(existing, []byte(anchorBegin))]
	if !bytes.HasPrefix(out, above) {
		t.Fatalf("bytes above the anchor changed:\n got prefix %q\n want %q", out[:len(above)], above)
	}

	below := existing[bytes.Index(existing, []byte(anchorEndLine))+len(anchorEndLine):]
	if !bytes.HasSuffix(out, below) {
		t.Fatalf("bytes below the insertion point changed:\n got suffix %q\n want %q",
			out[len(out)-len(below):], below)
	}
}

func TestInsertBlockAfter_UpdateInPlace(t *testing.T) {
	anchorBegin := BeginPrefix + "baseline-include"
	anchorEnd := EndPrefix + "baseline-include"
	authorBegin := BeginPrefix + "global-git-author"
	authorEnd := EndPrefix + "global-git-author"
	existing := []byte(
		anchorBegin + "\n[include]\n\tpath = ~/.gitconfig.d/00-baseline\n" + anchorEnd + "\n" +
			authorBegin + "\n[user]\n\tname = Old\n" + authorEnd + "\n" +
			"[includeIf \"gitdir:~/git/work/\"]\n\tpath = ~/.gitconfig.d/work\n",
	)
	origBeginOff := bytes.Index(existing, []byte(authorBegin))

	out, err := InsertBlockAfter(existing, "baseline-include", "global-git-author", "[user]\n\tname = New")
	if err != nil {
		t.Fatalf("InsertBlockAfter: %v", err)
	}
	gotBeginOff := bytes.Index(out, []byte(authorBegin))
	if gotBeginOff != origBeginOff {
		t.Errorf("in-place update moved the block: begin offset %d -> %d", origBeginOff, gotBeginOff)
	}
	if n := bytes.Count(out, []byte(authorBegin)); n != 1 {
		t.Errorf("expected exactly one block of that name, got %d", n)
	}
	if !bytes.Contains(out, []byte("name = New")) {
		t.Errorf("updated body missing:\n%s", out)
	}
	if bytes.Contains(out, []byte("name = Old")) {
		t.Errorf("old body still present:\n%s", out)
	}
}

func TestInsertBlockAfter_MissingAnchor(t *testing.T) {
	existing := []byte("[user]\n\tname = Alice\n")
	out, err := InsertBlockAfter(existing, "baseline-include", "global-git-author", "[user]\n\temail = a@b.com")
	if err == nil {
		t.Fatal("expected error naming the missing anchor")
	}
	if !strings.Contains(err.Error(), "baseline-include") {
		t.Errorf("error must name the missing anchor, got: %v", err)
	}
	if !bytes.Equal(out, existing) {
		t.Errorf("missing-anchor must return input bytes unchanged\n got %q\n want %q", out, existing)
	}
}

func TestInsertBlockAfter_CRLFAnchor(t *testing.T) {
	existing := []byte(
		BeginPrefix + "baseline-include\r\n" +
			"[include]\r\n\tpath = ~/.gitconfig.d/00-baseline\r\n" +
			EndPrefix + "baseline-include\r\n" +
			"[core]\r\n\tignorecase = false\r\n",
	)
	out, err := InsertBlockAfter(existing, "baseline-include", "global-git-author", "[user]\n\tname = Pat")
	if err != nil {
		t.Fatalf("CRLF InsertBlockAfter: %v", err)
	}
	if !bytes.Contains(out, []byte(BeginPrefix+"global-git-author\n")) {
		t.Errorf("inserted block missing after CRLF anchor:\n%q", out)
	}
	if !bytes.Contains(out, []byte("[core]\r\n\tignorecase = false\r\n")) {
		t.Errorf("foreign CRLF content below insertion was not preserved:\n%q", out)
	}
}
