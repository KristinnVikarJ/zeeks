package files

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var fileBufferMap = make(map[int]chan File)

func InitFileBuffer() {
	for i := 0; i < 1; i++ {
		fileBufferMap[i] = make(chan File, 50000)
		go processFileBuffer(i)
	}
}

func processFileBuffer(index int) {
	// log.Println("Starting print buffer nr:", index)
	outDir, ok := ArgMap["--outputDir"]
	if !ok {
		outDir = time.Now().Format("01-02-06-15-04-05")
	}
	var file File
	var err error
	var dir string
	var fn string
	var cloneFile *os.File

	for {
		file = <-fileBufferMap[index]
		dir, fn = filepath.Split(file.Name)
		err = os.MkdirAll(outDir+"/"+dir, 0777)
		if err != nil {
			GlobalWaitGroup.Done()
			log.Println(err)
			continue
		}
		cloneFile, err = os.OpenFile(outDir+"/"+dir+fn, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0777)
		if err != nil {
			GlobalWaitGroup.Done()
			log.Println(err)
			continue
		}

		for i, v := range file.Results.Hits {
			_, _ = cloneFile.WriteString("(" + strconv.Itoa(i) + "): " + v + "\n")
		}
		cloneFile.Close()
		GlobalWaitGroup.Done()
	}
}
