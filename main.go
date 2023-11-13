package main

import (
	"archive/zip"
	"compress/flate"
	"flag"
	"fmt"
	"github.com/mholt/archiver/v3"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//goland:noinspection ALL
const (
	RarArchiveExt        = ".rar"
	RarArchivePartialExt = "part\\+.rar"
	ZipArchiveExt        = ".zip"
	RarPattern           = "*.r??"
	ZipPattern           = "*.zip"
	RarPatternExtraOne   = "*.s*"
	NfoPattern           = ".nfo"
	IsoPattern           = ".iso"
	EpubPattern          = ".epub"
	PdfPattern           = ".pdf"
	MobiPattern          = ".mobi"
	AzwPattern           = ".azw"
	Azw3Pattern          = ".azw3"
	DizPattern           = ".diz"
	Mp4Pattern           = ".mp4"
	SfvPattern           = "*.sfv"
)

func main() {
	var (
		directoryToScan string
		outputDirectory string
		archiveType     string
		initialArch     string
	)
	flag.StringVar(&directoryToScan, "dir", "", "Directory with archives. (Required)")
	flag.StringVar(&outputDirectory, "out", "", "Output directory. Optional")
	flag.StringVar(&archiveType, "arch", "", "Archive formats to take care of, e.g.: 'rar,zip'. If none provided zip and rar will be searched for.")
	flag.StringVar(&initialArch, "initial_archive_type", "", "[To be considered / TBD] Defines if files are archived initially in some format and need to take special care (if you need to extract them in folders that are named after archives)")
	flag.Parse()

	if directoryToScan == "" {
		log.Fatal("source directory required")
	}

	sourceDirectoryExists, _ := directoryExists(directoryToScan)

	if !sourceDirectoryExists {
		log.Fatal("source directory does not exist")
	}

	archTypes := handleArchiveTypes(archiveType)
	start := time.Now()

	var (
		errorsWalking   = 0
		errorsUnpacking = 0
		failedToUnpack  []string
	)

	// unzip
	walkFilesX := WalkDirectories(directoryToScan, archTypes, &errorsWalking, &failedToUnpack)

	/*TEH WHILE!*/
	for len(walkFilesX) != 0 {
		handleArchives(walkFilesX, directoryToScan, &failedToUnpack, &errorsUnpacking)
		walkFilesX = WalkDirectories(directoryToScan, archTypes, &errorsWalking, &failedToUnpack)
	}

	log.Println("done extracting")

	log.Printf("cleaning up")

	azw3 := WalkDirectories(directoryToScan, []string{Azw3Pattern}, &errorsWalking, nil)
	azw3s := azw3[Azw3Pattern]
	if len(azw3s) > 0 {
		for i := range azw3s {
			renameInRelease(azw3s[i], outputDirectory, NfoPattern)
		}
	}

	azw := WalkDirectories(directoryToScan, []string{AzwPattern}, &errorsWalking, nil)
	azws := azw[AzwPattern]
	if len(azws) > 0 {
		for i := range azws {
			renameInRelease(azws[i], outputDirectory, AzwPattern)
		}
	}

	/*nfo := WalkDirectories(directoryToScan, []string{NfoPattern}, &errorsWalking)
	nfos := nfo[NfoPattern]
	if len(nfos) > 0 {
		for i := range nfos {
			renameInRelease(nfos[i], outputDirectory, NfoPattern)
		}
	}*/

	iso := WalkDirectories(directoryToScan, []string{IsoPattern}, &errorsWalking, nil)
	isos := iso[IsoPattern]
	if len(isos) > 0 {
		for i := range isos {
			renameInRelease(isos[i], outputDirectory, IsoPattern)
		}
	}
	epub := WalkDirectories(directoryToScan, []string{EpubPattern}, &errorsWalking, nil)
	epubs := epub[EpubPattern]
	if len(epubs) > 0 {
		for i := range epubs {
			renameInRelease(epubs[i], outputDirectory, EpubPattern)
		}
	}
	pdf := WalkDirectories(directoryToScan, []string{PdfPattern}, &errorsWalking, nil)
	pdfs := pdf[PdfPattern]
	if len(pdfs) > 0 {
		for i := range pdfs {
			renameInRelease(pdfs[i], outputDirectory, PdfPattern)
		}
	}
	mobi := WalkDirectories(directoryToScan, []string{MobiPattern}, &errorsWalking, nil)
	mobis := mobi[MobiPattern]
	if len(mobis) > 0 {
		for i := range mobis {
			renameInRelease(mobis[i], outputDirectory, MobiPattern)
		}
	}
	/*diz := WalkDirectories(directoryToScan, []string{DizPattern}, &errorsWalking)
	dizes := diz[DizPattern]
	if len(dizes) > 0 {
		for i := range dizes {
			renameInRelease(dizes[i], outputDirectory, DizPattern)
		}
	}*/
	removeEmptyDirectories(directoryToScan)
	elapsed := time.Since(start)
	log.Printf("listing took %s", elapsed)
	log.Printf("total errors walking: %d\n", errorsWalking)
	log.Printf("total errors unpacking: %d", errorsUnpacking)

}

func handleArchiveTypes(archiveType string) []string {
	var archTypes []string
	if len(archiveType) > 0 {
		archTypes = strings.Split(archiveType, ",")
	} else {
		archTypes = []string{"rar", "zip"}
	}

	archTypes = updateStrings(archTypes)
	log.Printf("archive types were set to: %s", archTypes)
	return archTypes
}

func handleArchives(walkFilesX map[string][]string, outputDirectory string, failedToUnpack *[]string, errorsUnpacking *int) {
	keys := getKeysFromMap(walkFilesX)
	for i := range keys {
		unpackAllFiles(walkFilesX[keys[i]], outputDirectory, keys[i], failedToUnpack, errorsUnpacking)
	}
}

func getKeysFromMap(mapOfElements map[string][]string) []string {
	keys := make([]string, 0, len(mapOfElements))
	for k := range mapOfElements {
		keys = append(keys, k)
	}
	return keys
}

func unpackAllFiles(files []string, outputDirectory string, ext string, failedToUnpack *[]string, errorsUnpacking *int) {

	rarArchiver := archiver.Rar{
		OverwriteExisting:      true,
		MkdirAll:               true,
		ImplicitTopLevelFolder: false,
		StripComponents:        0,
		ContinueOnError:        false,
	}

	zipArchiver := archiver.Zip{
		CompressionLevel:       flate.BestCompression,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ContinueOnError:        false,
		OverwriteExisting:      true,
		ImplicitTopLevelFolder: false,
	}

	for pos, file := range files {
		fileAbsolutePath, _ := filepath.Abs(file)
		var outputFullPath = getFullPath(outputDirectory, file)
		if strings.HasSuffix(file, RarArchivePartialExt) {
			continue
		}
		// check if file exists
		if _, err := os.Stat(file); os.IsNotExist(err) {
			*failedToUnpack = append(*failedToUnpack, file)
			*errorsUnpacking++
			continue
		} else {

			if ext == RarArchiveExt {
				handleRarArchive(rarArchiver, fileAbsolutePath, outputFullPath, file, failedToUnpack, errorsUnpacking, &files, pos)
				remove(files, pos)
			} else if ext == ZipArchiveExt {
				handleZipArchive(zipArchiver, fileAbsolutePath, outputFullPath, file, failedToUnpack, errorsUnpacking, &files, pos)
				remove(files, pos)
			}
		}
	}
}

// https://stackoverflow.com/a/37335777 god bless ya mate
func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func handleRarArchive(rarArchiver archiver.Rar, fileAbsolutePath string, outputFullPath string, file string, failedToUnpack *[]string, errorsUnpacking *int, i *[]string, pos int) {

	err := rarArchiver.Unarchive(fileAbsolutePath, outputFullPath)
	// check if file exists before unpacking it
	if _, err := os.Stat(outputFullPath); os.IsNotExist(err) {
		*failedToUnpack = append(*failedToUnpack, file)
		*errorsUnpacking++
		return
	}
	defer func(rarArchiver *archiver.Rar) {
		err := rarArchiver.Close()
		if err != nil {
			log.Fatalf("failed to close file: %s, %v\n", file, err)
		}
	}(&rarArchiver)
	if err != nil {
		log.Printf("failed to unpack file: %s, %v\n", file, err)
	} else {
		//log.Printf("file was unpacked: %s\n", file)
		forGlob := filepath.Join(filepath.Dir(file), RarPattern)
		files, err := filepath.Glob(forGlob)
		if err != nil {
			log.Printf("failed to make glob pattern: %s, %v", forGlob, err)
		}
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				log.Printf("failed to remove file: %s, %v", file, err)
			} /*else {
				//log.Printf("file was removed: %s", file)
			}*/
		}
	}
}

func handleZipArchive(zipArchiver archiver.Zip, fileAbsolutePath string, outputFullPath string, file string, unpack *[]string, unpacking *int, i *[]string, pos int) {
	err := zipArchiver.Unarchive(fileAbsolutePath, outputFullPath)
	if err != nil {
		log.Printf("failed to unpack file: %s, %v\n", file, err)
	} else {
		//log.Printf("file was unpacked: %s\n", file)
		err := os.Remove(file)
		if err != nil {
			log.Printf("failed to remove file: %s, %v", file, err)
		} /*else {
			log.Printf("file was removed: %s", file)
		}*/
	}
}
func getFullPath(outputDirectory string, file string) string {
	if outputDirectory == "" {
		return filepath.Dir(file)
	} else {
		return filepath.Join(outputDirectory, filepath.Base(filepath.Dir(file)))
	}
}

// unzipInDirectory extracts the files from a zip file into a specified directory.
//
//goland:noinspection ALL
func unzipInDirectory(zipFile string) {
	// Get the output directory path
	outputDirectory := filepath.Dir(zipFile)

	// Open the zip file for reading
	archiveReader, err := zip.OpenReader(zipFile)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := archiveReader.Close(); err != nil {
			panic(err)
		}
	}()

	// Iterate over each file in the zip archive
	for _, f := range archiveReader.File {
		// Initialize a flag to track if the file was successfully unzipped
		var ok = false

		// Get the file path for the current file
		filePath := filepath.Join(outputDirectory, f.Name)
		fmt.Println("Unzipping file:", filePath)

		// Check if the file path is valid
		if !strings.HasPrefix(filePath, filepath.Clean(outputDirectory)+string(os.PathSeparator)) {
			fmt.Println("Invalid file path")
			return
		}

		// If the file is a directory, create it
		if f.FileInfo().IsDir() {
			fmt.Println("Creating directory...")
			err := os.MkdirAll(filePath, os.ModePerm)
			if err != nil {
				_ = fmt.Errorf("Failed to create directory: %s", err)
			}
			continue
		}

		// Create the parent directory of the file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			panic(err)
		}

		// Open the destination file for writing
		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			panic(err)
		}

		// Open the file in the zip archive for reading
		fileInArchive, err := f.Open()
		if err != nil {
			panic(err)
		}

		// Copy the contents of the file in the archive to the destination file
		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			panic(err)
		}

		// Set the flag to indicate successful unzipping
		ok = true

		// Close the destination file
		err = dstFile.Close()
		if err != nil {
			ok = false
			log.Fatal(err)
		}

		// Close the file in the archive
		err = fileInArchive.Close()
		if err != nil {
			ok = false
			log.Fatal(err)
		}

		// If the file was successfully unzipped, remove the zip file
		if ok {
			err := os.Remove(zipFile)
			if err != nil {
				log.Fatalf("Failed to remove file: %s, %v\n", zipFile, err)
			}
		}
	}
}

func updateStrings(archTypes []string) []string {
	var archTypesNew []string
	for _, el := range archTypes {
		archTypesNew = append(archTypesNew, "."+el)
	}
	return archTypesNew
}

//goland:noinspection ALL
func replaceStringAtIndex(in string, r rune, i int) string {
	out := []rune(in)
	out[i] = r
	return string(out)
}

//goland:noinspection ALL
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
	//log.Printf("going to unpack file: %s", archive)
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
			renameInRelease(archive, "", "")
			delete(*files, archive)
			return
		}
		epub, _ := listFilesWithPattern(archive, EpubPattern)
		if len(epub) > 0 {
			cleanUpAfterUnpack(archive)
			renameInRelease(archive, "", ".epub")
			renameInRelease(archive, "", ".nfo")
			delete(*files, archive)
			return
		}
		pdf, _ := listFilesWithPattern(archive, PdfPattern)
		if len(pdf) > 0 {
			cleanUpAfterUnpack(archive)
			renameInRelease(archive, "", ".pdf")
			renameInRelease(archive, "", ".nfo")
			delete(*files, archive)
			return
		}
		mobi, _ := listFilesWithPattern(archive, MobiPattern)
		if len(mobi) > 0 {
			cleanUpAfterUnpack(archive)
			renameInRelease(archive, "", ".mobi")
			renameInRelease(archive, "", ".nfo")
			delete(*files, archive)
			return
		}
		mp4, _ := listFilesWithPattern(archive, Mp4Pattern)
		if len(mp4) > 0 {
			cleanUpAfterUnpack(filepath.Dir(archive))
			delete(*files, archive)
			return
		} else {
			dir, err := os.ReadDir(filepath.Dir(archive))
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

//goland:noinspection ALL
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
func renameInRelease(file string, directoryToScan string, patternString string) {
	filePath := filepath.Dir(file)
	newFileName := filepath.Base(filePath)
	//parentDirectory := filepath.Dir(filePath)
	join := filepath.Join(directoryToScan, newFileName+patternString)
	err := os.Rename(file, join)
	if err != nil {
		log.Printf("Error renaming %s to %s\n%v\n", file, join, err)
	}
}

func cleanUpAfterUnpack(rarFile string) {
	for i := range []string{RarPattern, SfvPattern} {
		filesWithPattern, err := listFilesWithPattern(rarFile, []string{RarPattern, SfvPattern}[i])
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

func removeEmptyDirectories(folder string) {
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			pattern, _ := listFilesWithPattern(path, "*")
			if len(pattern) <= 0 {
				log.Printf("empty: %s", path)
				err := os.Remove(path)
				if err != nil {
					fmt.Printf("error deleting empty directory: %s, %v", path, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking directory: %s, %v", folder, err)
	}
}

func listFilesWithPattern(folder string, pattern string) ([]string, error) {
	join := filepath.Join(filepath.Clean(folder), pattern)
	files, err := filepath.Glob(join)
	if err != nil {
		log.Printf("could not find files with pattern %s in %s\n%v", pattern, folder, err)
	}
	return files, err
}

func WalkDirectories(root string, extensions []string, errorsWalking *int, failedToUnpack *[]string) map[string][]string {
	var directories = make(map[string][]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		var ext = filepath.Ext(path)
		if failedToUnpack != nil && ArrayContains(*failedToUnpack, path) {
			return filepath.SkipDir
		}
		if !info.IsDir() && ArrayContains(extensions, ext) {
			if value, ok := directories[ext]; ok {
				directories[ext] = append(value, path)
			} else {
				directories[ext] = []string{path}
			}
		}
		return nil
	})
	if err != nil {
		*errorsWalking++
		log.Printf("error walking directory '%s'\n%v", root, err)
	}
	return directories
}

//goland:noinspection ALL
func mapToSet(map[string]types.Tuple) map[string]bool {
	var set = make(map[string]bool) // New empty set
	set["Foo"] = true               // Add
	for k := range set {            // Loop
		fmt.Println(k)
	}
	return set
}

//goland:noinspection ALL
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

// exists returns whether the given file or directory exists
func directoryExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
