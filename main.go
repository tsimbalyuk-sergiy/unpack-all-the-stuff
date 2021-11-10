package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"github.com/mholt/archiver/v3"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RarPattern         = "*.r*"
	RarPatternExtraOne = "*.s*"
	IsoPattern         = "*.iso"
	EpubPattern        = "*.epub"
	PdfPattern         = "*.pdf"
	MobiPattern        = "*.mobi"
	Mp4Pattern         = "*.mp4"
	SfvPattern         = "*.sfv"
)

var list = []string{RarPattern, SfvPattern}

func main() {
	var (
		directoryToScan string
		outputDirectory string
		archiveType     string
	)
	flag.StringVar(&directoryToScan, "dir", "", "Directory with archives. (Required)")
	flag.StringVar(&outputDirectory, "out", "", "Output directory. Optional")
	flag.StringVar(&archiveType, "arch", "", "Archive formats to take care of, e.g.: 'rar,zip'. If none provided zip and rar will be searched for.")
	flag.Parse()

	if directoryToScan == "" {
		log.Fatal("Please specify target directory")
	}
	/*if outputDirectory == "" {
		outputPath := filepath.Join(directoryToScan, "out_"+time.Now().Format("150405"))
		err := os.Mkdir(outputPath, 0755)
		if err != nil {
			panic(err)
		} else {
			log.Println("Output directory was set to: " + outputPath)
		}
	}*/
	var archTypes []string
	if len(archiveType) > 0 {
		archTypes = strings.Split(archiveType, ",")
	} else {
		archTypes = []string{"rar", "zip"}
	}

	archTypes = updateStrings(archTypes)
	log.Printf("archive types were set to: %s", archTypes)
	start := time.Now()

	var (
		errorsWalking   = 0
		errorsUnpacking = 0
	)
	//var files = WalkFiles(directoryToScan /*[]string{".rar"}*/, archTypes, &errorsWalking)
	walkFilesX := WalkDirectories(directoryToScan, archTypes, &errorsWalking)
	//sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i]) < strings.ToLower(files[j]) })
	for i := range walkFilesX {
		archiver.Walk(walkFilesX[i], func(f archiver.File) error {
			zfh, ok := f.Header.(zip.FileHeader)
			if ok {
				fmt.Println("Filename:", zfh.Name)
			}
			return nil
		})
	}
	//for i := range files {
	//for ok := true; ok; ok = len(files) > 0 {
	//
	//}

	elapsed := time.Since(start)
	log.Printf("listing took %s", elapsed)
	log.Printf("total errors walking: %d\n", errorsWalking)
	log.Printf("total errors unpacking: %d", errorsUnpacking)

}

func updateStrings(archTypes []string) []string {
	var archTypesNew []string
	for _, el := range archTypes {
		archTypesNew = append(archTypesNew, "."+el)
	}
	return archTypesNew
}

func replaceStringAtIndex(in string, r rune, i int) string {
	out := []rune(in)
	out[i] = r
	return string(out)
}

func unpackInDirectory(errorsUnpacking *int, files *map[string]bool) {
	var archive string
	for k := range *files {
		archive = k
	}
	extension, _ := archiver.ByExtension(archive)
	switch v := extension.(type) {
	case *archiver.Rar:
		v.OverwriteExisting = true
		v.ImplicitTopLevelFolder = false
	case *archiver.Zip:
		v.OverwriteExisting = true
	case *archiver.Tar:
		v.OverwriteExisting = true
		v.ImplicitTopLevelFolder = false
	case *archiver.TarGz:
	case *archiver.TarBz2:
		v.Tar.OverwriteExisting = true
		v.Tar.ImplicitTopLevelFolder = false
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovering from panic in unpackInDirectory error is: %v \n", r)
		}
	}()
	log.Printf("going to unpack file: %s", archive)
	unarch := extension.(archiver.Unarchiver)
	err := unarch.Unarchive(archive, filepath.Dir(archive))
	if err != nil {
		*errorsUnpacking++
		log.Printf("error unpacking file %s\n%v", archive, err)
		panic(err)
	} else {
		//do not delete files until we are completely sure we have extracted something actually
		log.Printf("cleaning up")
		iso, _ := listFilesWithPattern(archive, IsoPattern)
		if len(iso) > 0 {
			cleanUpAfterUnpack(archive)
			renameIsoInRelease(archive)
			renameInRelease(archive, ".nfo")
			delete(*files, archive)
			return
		}
		epub, _ := listFilesWithPattern(archive, EpubPattern)
		if len(epub) > 0 {
			cleanUpAfterUnpack(archive)
			renameInRelease(archive, ".epub")
			renameInRelease(archive, ".nfo")
			delete(*files, archive)
			return
		}
		pdf, _ := listFilesWithPattern(archive, PdfPattern)
		if len(pdf) > 0 {
			cleanUpAfterUnpack(archive)
			renameInRelease(archive, ".pdf")
			renameInRelease(archive, ".nfo")
			delete(*files, archive)
			return
		}
		mobi, _ := listFilesWithPattern(archive, MobiPattern)
		if len(mobi) > 0 {
			cleanUpAfterUnpack(archive)
			renameInRelease(archive, ".mobi")
			renameInRelease(archive, ".nfo")
			delete(*files, archive)
			return
		}
		mp4, _ := listFilesWithPattern(archive, Mp4Pattern)
		if len(mp4) > 0 {
			cleanUpAfterUnpack(filepath.Dir(archive))
			delete(*files, archive)
			return
		} else {
			dir, err := ioutil.ReadDir(filepath.Dir(archive))
			if err != nil {
				log.Printf("Error reading directory %s\n", archive)
				delete(*files, archive)
				return
			}
			var hasSubdirectories = false
			for file := range dir {
				if dir[file].IsDir() {
					hasSubdirectories = true
				}
			}
			if hasSubdirectories {
				cleanUpAfterUnpack(archive)
				delete(*files, archive)
			}
		}
		switch extension.(type) {
		case *archiver.Zip:
			possibleRars, _ := listFilesWithPattern(archive, "*.rar")
			log.Printf("possible rars: %v", possibleRars)
			if len(possibleRars) > 0 {
				for i := range possibleRars {
					rarString := possibleRars[i]
					if !(*files)[rarString] {
						(*files)[rarString] = true
					}
				}
			}
			err := os.Remove(archive)
			if err != nil {
				log.Printf("error deleting %s: %v", archive, err)
			}
		case *archiver.Rar:

		}
	}
	delete(*files, archive)
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func renameIsoInRelease(rar string) {
	newIsoName := filepath.Join(filepath.Dir(rar), strings.TrimSuffix(filepath.Base(rar), filepath.Ext(rar))+".iso")
	join := filepath.Join(filepath.Dir(rar), filepath.Base(filepath.Dir(rar))+".iso")
	err := os.Rename(newIsoName, join)
	if err != nil {
		log.Printf("Error renaming %s to %s\n%v\n", rar, join, err)
	}
}
func renameInRelease(rar string, patternString string) {
	pattern, err2 := listFilesWithPattern(rar, "*"+patternString)

	if len(pattern) > 0 && err2 == nil {
		join := filepath.Join(filepath.Dir(rar), filepath.Base(filepath.Dir(rar))+patternString)
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

func WalkFiles(root string, extensions []string, errorsWalking *int) map[string]bool {
	var filesMap = make(map[string]bool)
	//var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		var ext = filepath.Ext(path)
		if !info.IsDir() && ArrayContains(extensions, ext) {
			//fmt.Printf("extension is %s of %s\n", ext, path)
			//fmt.Printf("path: %s\n", path)
			//files = append(files, path)
			filesMap[path] = true
		}
		return nil
	})
	if err != nil {
		*errorsWalking++
		log.Printf("error walking directory '%s'\n%v", root, err)
	}
	return filesMap
}

func WalkDirectories(root string, extensions []string, errorsWalking *int) []string {
	var directories []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		var ext = filepath.Ext(path)
		if !info.IsDir() && ArrayContains(extensions, ext) {
			//fmt.Printf("extension is %s of %s\n", ext, path)
			//fmt.Printf("path: %s\n", path)
			directories = append(directories, path)
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
