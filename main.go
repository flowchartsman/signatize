package main

import (
	"fmt"
	"github.com/unidoc/unidoc/common/license"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pborman/getopt/v2"
	pdf "github.com/unidoc/unidoc/pdf/model"
)

func main() {
	log.SetFlags(0)

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalln("can't get working directory somehow", err)
	}

	leavesPer := getopt.IntLong("leaves", 'l', 4, "number of leaves per signature")
	rawsplits := getopt.ListLong("split-pages", 'p', "page numbers marking splits into different books")
	signatureFiles := getopt.BoolLong("signature-files", 'f', "if true, signitize will print a separate pdf for each folio")
	outputDir := getopt.String('o', cwd, "the file to put the generated pdf files (or directories if used with -f)")

	getopt.Parse()

	inputPath := getopt.Arg(0)
	if inputPath == "" {
		log.Fatal("must provide filename")
	}

	if *leavesPer < 2 {
		log.Fatalf("invalid value %q for leaves. Must be >= 2", *leavesPer)
	}

	// TODO: new variable
	*leavesPer = *leavesPer * 4

	var splits []int
	if len(*rawsplits) > 0 {
		splits = make([]int, len(*rawsplits))
		for i, s := range *rawsplits {
			ival, err := strconv.Atoi(s)
			if err != nil {
				log.Fatalf("invalid split value %q, must be an int", s)
			}
			splits[i] = ival
		}
	}

	// ensure output directory exists.  If not, create it
	ensureDir(*outputDir)

	// load the PDF
	inputFile, err := os.Open(inputPath)
	if err != nil {
		log.Fatalln("error opening input file", err)
	}
	reader, err := pdf.NewPdfReader(inputFile)
	if err != nil {
		log.Fatalln("error opening pdf", err)
	}
	isEncrypted, err := reader.IsEncrypted()
	if err != nil {
		log.Fatalln("error checking pdf encryption", err)
	}
	if isEncrypted {
		auth, err := reader.Decrypt([]byte(""))
		if err != nil {
			log.Fatalln("error decrypting document", err)
		}
		if !auth {
			log.Fatalln("cannot signitize encrypted document (yet!)")
		}
	}

	// get all the pages
	numPages, err := reader.GetNumPages()
	println("numpages", numPages)
	if err != nil {
		log.Fatalln("couldn't get page count", err)
	}
	if numPages == 0 {
		log.Fatalf("no pages found in pdf")
	}

	pages := make([]*pdf.PdfPage, numPages)
	for i := 1; i <= numPages; i++ {
		page, err := reader.GetPage(i)
		if err != nil {
			log.Fatalf("error getting page %d from %q: %v", i, inputFile.Name(), err)
		}
		pages[i-1] = page
	}

	// if the user doesn't want to split, we make the "first" split the last
	// page, so the rest of the logic works
	if len(splits) == 0 {
		println("splits len is", numPages)
		splits = []int{numPages}
	}

	// create a blank page based on the dimensions of the first
	blankPage := pdf.NewPdfPage()
	blankPage.MediaBox = &pdf.PdfRectangle{
		Llx: pages[0].MediaBox.Llx,
		Lly: pages[0].MediaBox.Lly,
		Urx: pages[0].MediaBox.Urx,
		Ury: pages[0].MediaBox.Ury,
	}
	blankPage.Resources = pdf.NewPdfPageResources()

	// split the pdf into volumes, depending
	volumes := make([][]*pdf.PdfPage, len(splits))
	last := 0
	for i, splitpage := range splits {
		volume := pages[last:splitpage]
		neededPages := len(volume) % *leavesPer % 4
		if neededPages != 0 {
			v := make([]*pdf.PdfPage, len(volume), len(volume)+neededPages)
			copy(v, volume)
			for j := 0; j < neededPages; j++ {
				v = append(v, blankPage)
			}
			volume = v
		}
		volumes[i] = volume
		last = splitpage + 1
	}

	// basis of name to generate filenames from
	basename := filepath.Base(inputPath)
	basename = basename[0 : len(basename)-len(filepath.Ext(basename))]
	basename = filepath.Join(*outputDir, basename) + "-out"

	writer := pdf.NewPdfWriter()
	var signatureNumber int
	var output *os.File

	for vi, volume := range volumes {
		signatureNumber = 1

		var (
			vName string
			sName string
			flip  bool
		)

		if len(volumes) > 1 {
			vName = fmt.Sprintf("-%03d", vi+1)
		}

		if *signatureFiles {
			sName = fmt.Sprintf("-%03d", signatureNumber)
		}

		output = getOutputFile(basename, vName, sName)
		println("output", output.Name())

		println("len volume", len(volume))
		for i := 0; i < len(volume); i += *leavesPer {
			var signature []*pdf.PdfPage
			println("i", i)
			if len(volume)-i < *leavesPer {
				signature = volume[i:]
			} else {
				signature = volume[i : i+*leavesPer]
			}
			println("len signature", len(signature))
			for j := 0; j < len(signature)/2; j++ {
				var (
					first  *pdf.PdfPage
					second *pdf.PdfPage
				)
				flip = !flip
				if flip {
					println("first", len(signature)-1-j, "second", j)
					first = signature[len(signature)-1-j]
					second = signature[j]
				} else {
					println("first", j, "second", len(signature)-1-j)
					first = signature[j]
					second = signature[len(signature)-1-j]
				}
				/*
					page := pdf.NewPdfPage()
					page.MediaBox = &pdf.PdfRectangle{
						Llx: blankPage.MediaBox.Llx,
						Lly: blankPage.MediaBox.Lly,
						Urx: blankPage.MediaBox.Urx,
						Ury: blankPage.MediaBox.Ury,
					}
				*/
				writer.AddPage(first)
				writer.AddPage(second)
			}
			// writewrite
			if *signatureFiles {
				// if we're writing individual files, close this file and start
				// the next one
				writer.Write(output)
				output.Close()
				writer = pdf.NewPdfWriter()
				signatureNumber++
				sName = fmt.Sprintf("-%03d", signatureNumber)
				output = getOutputFile(basename, vName, sName)
			}
		}
		if !*signatureFiles {
			writer.Write(output)
			output.Close()
		}
	}

}

func ensureDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.Mkdir(path, os.ModePerm); err != nil {
			log.Fatalf("Cannot create output directory %q: %v", path, err)
		}
	}
}

func getOutputFile(basename, vname, sname string) *os.File {
	fileName := fmt.Sprintf("%s%s%s.pdf", basename, vname, sname)
	f, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("error creating file %q:%v", fileName, err)
	}
	return f
}
