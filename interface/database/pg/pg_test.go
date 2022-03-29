package pg

import "testing"

func testParseLike(t *testing.T, unparsedValue, valueExp, opExp string) {
	value, op := parseLike(unparsedValue)
	if value != valueExp {
		t.Errorf("%s: Expect value: %s, have %s", unparsedValue, valueExp, value)
	}
	if op != opExp {
		t.Errorf("%s: Expect operator: %s, have %s", unparsedValue, opExp, op)
	}
}

func TestParseLike(t *testing.T) {
	testParseLike(t, "test", "test", "=")
	testParseLike(t, "test_test", `test_test`, "=")
	testParseLike(t, "test*test", `test%test`, "LIKE")
	testParseLike(t, "test*test_", `test%test\_`, "LIKE")
	testParseLike(t, "test?test_", `test_test\_`, "LIKE")
	testParseLike(t, "test(?i)", `test`, "ILIKE")
	testParseLike(t, "test_test(?i)", `test\_test`, "ILIKE")
	testParseLike(t, "test*test(?i)", `test%test`, "ILIKE")
	testParseLike(t, "test*test_(?i)", `test%test\_`, "ILIKE")
	testParseLike(t, "test?test_(?i)", `test_test\_`, "ILIKE")
}
