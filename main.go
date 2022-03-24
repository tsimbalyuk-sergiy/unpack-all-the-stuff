package main

import (
	"flag"
	"fmt"
	"github.com/mholt/archiver/v3"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	RarPattern = "*.r*"
	IsoPattern = "*.iso"
	Mp4Pattern = "*.mp4"
	SfvPattern = "*.sfv"
)

var list = []string{RarPattern, SfvPattern}

func main() {
	var errorsWalking = 0
	var errorsUnpacking = 0

	//var unpack string
	var directoryToScan string
	flag.StringVar(&directoryToScan, "dir", "", "Directory to scan. (Required)")
	//var unpack string
	//flag.StringVar(&unpack, "unpack", "", "Unpack action. (Required)")
	flag.Parse()

	if directoryToScan == "" {
		log.Fatal("Please specify target directory")
	}

	start := time.Now()

	var files = WalkFiles(directoryToScan, []string{".rar"}, &errorsWalking)
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i]) < strings.ToLower(files[j]) })

	for i := range files {
		unrarInDirectory(files[i], &errorsUnpacking)
	}

	elapsed := time.Since(start)
	log.Printf("listing took %s", elapsed)
	log.Printf("total errors walking: %d\n", errorsWalking)
	log.Printf("total errors unpacking: %d", errorsUnpacking)

}

func unrarInDirectory(folderWithRar string, errorsUnpacking *int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovering from panic in unrarInDirectory error is: %v \n", r)
		}
	}()
	log.Printf("going to unpack files in directory: %s", folderWithRar)
	err := archiver.Unarchive(folderWithRar, filepath.Dir(folderWithRar))
	if err != nil {
		*errorsUnpacking++
		fmt.Printf("error unpacking file %s\n%v", folderWithRar, err)
		panic(err)
	} else {
		//do not delete files until we are completely sure we have extracted something actually
		log.Printf("cleaning up")
		iso, _ := listFilesWithPattern(folderWithRar, IsoPattern)
		if len(iso) > 0 {
			cleanUpAfterUnpack(folderWithRar)
			renameIsoInRelease(folderWithRar)
			renameNfoInRelease(folderWithRar)
			return
		}
		mp4, _ := listFilesWithPattern(folderWithRar, Mp4Pattern)
		if len(mp4) > 0 {
			cleanUpAfterUnpack(filepath.Dir(folderWithRar))
			return
		} else {
			dir, err := ioutil.ReadDir(filepath.Dir(folderWithRar))
			if err != nil {
				log.Printf("Error reading directory %s\n", folderWithRar)
				return
			}
			var hasSubdirectories = false
			for file := range dir {
				if dir[file].IsDir() {
					hasSubdirectories = true
				}
			}
			if hasSubdirectories {
				cleanUpAfterUnpack(folderWithRar)
			}
		}
	}
}

func renameIsoInRelease(rar string) {
	newIsoName := filepath.Join(filepath.Dir(rar), strings.TrimSuffix(filepath.Base(rar), filepath.Ext(rar))+".iso")
	join := filepath.Join(filepath.Dir(rar), filepath.Base(filepath.Dir(rar))+".iso")
	err := os.Rename(newIsoName, join)
	if err != nil {
		log.Printf("Error renaming %s to %s\n%v\n", rar, join, err)
	}
}

func renameNfoInRelease(rar string) {
	const nfo = ".nfo"
	pattern, err2 := listFilesWithPattern(rar, "*.nfo")

	if len(pattern) > 0 && err2 == nil {
		join := filepath.Join(filepath.Dir(rar), filepath.Base(filepath.Dir(rar))+nfo)
		err := os.Rename(pattern[0], join)
		if err != nil {
			log.Printf("Error renaming %s to %s\n%v\n", rar, join, err)
		}
	} else {
		log.Printf("Error renaming nfo file in %s\n%v\n", rar, err2)
	}
}

func cleanUpAfterUnpack(rarFile string) {
	for i := range list {
		filesWithPattern, err := listFilesWithPattern(rarFile, list[i])
		if err != nil {
			log.Printf("no files with filesWithPattern: %v\n", filesWithPattern)
		} else if len(filesWithPattern) > 0 {
			for fileToDelIndex := range filesWithPattern {
				log.Printf("will remove: %s\n", filesWithPattern[fileToDelIndex])
				err := os.Remove(filesWithPattern[fileToDelIndex])
				if err != nil {
					log.Printf("error deleting file: %s", filesWithPattern[fileToDelIndex])
				}
			}
		}
	}
}

func listFilesWithPattern(folder string, pattern string) ([]string, error) {
	join := filepath.Join(filepath.Dir(folder), pattern)
	files, err := filepath.Glob(join)
	if err != nil {
		log.Printf("could not find files with pattern %s in %s\n%v", pattern, folder, err)
	}
	return files, err
}

func WalkFiles(root string, extensions []string, errorsWalking *int) []string {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		var ext = filepath.Ext(path)
		if !info.IsDir() && ArrayContains(extensions, ext) {
			//fmt.Printf("extension is %s of %s\n", ext, path)
			//fmt.Printf("path: %s\n", path)
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		*errorsWalking++
		log.Printf("error walking directory '%s'\n%v", root, err)
	}
	return files
}

func WalkDirectories(root string, extensions []string, errorsWalking *int) []string {
	var directories []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		var ext = filepath.Ext(path)
		if !info.IsDir() && ArrayContains(extensions, ext) {
			//fmt.Printf("extension is %s of %s\n", ext, path)
			//fmt.Printf("path: %s\n", path)
			directories = append(directories, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		*errorsWalking++
		log.Printf("error walking directory '%s'\n%v", root, err)
	}
	return unique(directories)
}

func mapToSet(map[string]types.Tuple) map[string]bool {
	var set = make(map[string]bool) // New empty set
	set["Foo"] = true               // Add
	for k := range set {            // Loop
		fmt.Println(k)
	}
	return set

}

func unique(e []string) []string {
	var r []string

	for _, s := range e {
		if !contains(r[:], s) {
			r = append(r, s)
		}
	}
	return r
}

func contains(e []string, c string) bool {
	for _, s := range e {
		if s == c {
			return true
		}
	}
	return false
}

func ArrayContains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}
