package filesystem

import (
	"context"
	"os"
	"testing"

	geocubeStorage "github.com/airbusgeo/geocube/interface/storage"
)

func TestDelete(t *testing.T) {
	ctx := context.Background()
	f, err := os.CreateTemp("", "sample")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())
	s, err := NewFileSystemStrategy(ctx)
	if err != nil {
		panic(err)
	}
	err = s.Delete(ctx, f.Name())
	if err != nil {
		t.Errorf("Expecting nil error, found %v", err)
	}
	err = s.Delete(ctx, f.Name())
	if err == nil {
		t.Errorf("Expecting error, found nil")
	}
	err = s.Delete(ctx, f.Name(), geocubeStorage.IgnoreNotFound())
	if err != nil {
		t.Errorf("Expecting nil error, found %v", err)
	}
}

func TestBulkDelete(t *testing.T) {
	ctx := context.Background()
	dname, err := os.MkdirTemp("", "sampledir")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dname)

	var files []string
	for range 200 {
		f, err := os.CreateTemp(dname, "sample")
		if err != nil {
			panic(err)
		}
		files = append(files, f.Name())
		defer os.Remove(f.Name())
	}
	for _, f := range files[10:20] {
		if err = os.Remove(f); err != nil {
			panic(err)
		}
	}
	s := fileSystemStrategy{}
	if err = s.BulkDelete(ctx, files, geocubeStorage.IgnoreNotFound()); err != nil {
		t.Errorf("Expecting nil error, found %v", err)
	}
	for _, f := range files {
		if _, err := os.Stat(f); os.IsExist(err) {
			t.Errorf("File not deleted %s", f)
		}
	}

}
