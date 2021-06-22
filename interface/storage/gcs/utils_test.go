package gcs

import (
	"testing"
)

func TestParse(t *testing.T) {
	test := func(u string, mustErr bool, expb, expo string) {
		t.Helper()
		b, o, err := Parse(u)
		if mustErr {
			if err == nil {
				t.Error("error not raised")
			}
			return
		}
		if err != nil && !mustErr {
			t.Error(err)
			return
		}
		if b != expb {
			t.Errorf("got bucket \"%s\" expected \"%s\"", b, expb)
		}
		if o != expo {
			t.Errorf("got object \"%s\" expected \"%s\"", o, expo)
		}
	}
	//Successes
	test("gs://bucket/object.foo", false, "bucket", "object.foo")
	test("/bucket/object.foo", false, "bucket", "object.foo")
	test("bucket/object.foo", false, "bucket", "object.foo")

	test("gs://bucket/path/to/object.foo", false, "bucket", "path/to/object.foo")
	test("/bucket/path/to/object.foo", false, "bucket", "path/to/object.foo")
	test("bucket/path/to/object.foo", false, "bucket", "path/to/object.foo")

	//Failures
	test("bucket", true, "", "")
	test("bucket/", true, "", "")
	test("/bucket/", true, "", "")
	test("gs://bucket", true, "", "")
	test("gs://bucket/", true, "", "")
	test("//path/to/object", true, "", "")
	test("gs:///path/to/object", true, "", "")
}
