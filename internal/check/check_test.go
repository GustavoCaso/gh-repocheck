package check

import "testing"

func TestStatusString(t *testing.T) {
	cases := []struct {
		s    Status
		want string
	}{
		{Unknown, "unknown"}, {Pass, "pass"}, {Fail, "fail"}, {Warn, "warn"}, {Skip, "skip"},
		{Status(0), "unknown"}, // zero value must not be a valid status
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("Status.String() = %q, want %q", got, c.want)
		}
	}
}

func TestRepoFullName(t *testing.T) {
	r := Repo{Owner: "octocat", Name: "hello-world"}
	if got := r.FullName(); got != "octocat/hello-world" {
		t.Errorf("FullName() = %q, want %q", got, "octocat/hello-world")
	}
}
