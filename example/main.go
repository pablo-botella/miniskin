package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/pablo-botella/miniskin"
)

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(thisFile), "..", "testdata")

	ms := miniskin.MiniskinNew(testdata, testdata)

	result, err := ms.Run()
	if err != nil {
		log.Fatalf("miniskin: %v", err)
	}

	fmt.Printf("Embed file: %s (package %s)\n", result.BucketList.Filename, result.BucketList.Module)

	for _, br := range result.Buckets {
		fmt.Printf("\nBucket: %s -> %s (module: %s)\n", br.Bucket.Src, br.Bucket.Dst, br.Bucket.ModuleName)
		for _, item := range br.Items {
			processed := ""
			if item.NeedsProcessing() {
				processed = " [processed]"
			}
			key := item.Key
			if key == "" {
				key = "-"
			}
			fmt.Printf("  file=%-20s type=%-25s key=%-20s%s\n", item.File, item.Type, key, processed)
		}
	}
}
