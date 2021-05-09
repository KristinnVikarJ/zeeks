package files

import (
	"bufio"
	"bytes"
	"log"
	"math/rand"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/karrick/godirwalk"
)

var searchBufferMap = make(map[int]chan File)

func InitSearchBuffers() {
	number, err := strconv.Atoi(ArgMap["--concurrent"])
	if err != nil {
		log.Println("--concurrent needs to be a number")
		os.Exit(1)
	}
	// TODO flag to control the number of file buffers
	for i := 0; i < number; i++ {
		log.Println("Strating concurrent buffer number:", i)
		searchBufferMap[i] = make(chan File, 100000)
		go processSearchBuffer(i)
	}
}

func processSearchBuffer(index int) {
	// log.Println("Starting search buffer nr:", index)

	number, err := strconv.Atoi(ArgMap["--timeout"])
	if err != nil {
		log.Println("--timeout needs to be a number")
		os.Exit(1)
	}
	for {
		time.Sleep(time.Duration(number) * time.Millisecond)
		// TODO enable throttling for checks
		Search(<-searchBufferMap[index])
	}
}

func Search(v File) {
	var file *os.File
	var readyToUnlock bool
	defer func() {
		if r := recover(); r != nil {
			if file != nil {
				log.Println("error in file", file.Name())
			}
			log.Println("Panic while parsing file", r)
			log.Println(string(debug.Stack()))
		}
		if file != nil {
			file.Close()
		}
		if readyToUnlock {
			GlobalWaitGroup.Done()
		}
	}()

	stat, err := os.Stat(v.Name)
	if err != nil {
		log.Println("could not stat file", v.Name, err)
		readyToUnlock = true
		return
	}

	// if the file is 200mb or bigger, we continue
	if stat.Size() > RuntimeConfig.MaxFileSize*1000000 {
		readyToUnlock = true
		return
	}

	// ...
	for _, x := range RuntimeConfig.Ignore {
		if strings.Contains(v.Name, x) {
			readyToUnlock = true
			return
		}
	}
	// TODO.. detect binary file and open with strings to get output
	// note: don't forget to disable the file open below if it's a binary..

	file, err = os.Open(v.Name)
	if err != nil {
		log.Println("Can not open file", v.Name, err)
		readyToUnlock = true
		return
	}
	log.Println("opened file:", v.Name)

	scanner := bufio.NewScanner(file)
	var line string
	var lineBytes []byte
	lineNumber := 1
	var foundKeyword = false
	for scanner.Scan() {
		line = scanner.Text()
		lineBytes = scanner.Bytes()
		for _, c := range RuntimeConfig.ParsedConfigs {
			if c.String != "" && strings.Contains(line, c.String) {
				foundKeyword = true
				v.Results.Hits[strconv.Itoa(lineNumber)+":"+c.Prefix] = line
			}
			if c.Regexp != "" {
				match, err := regexp.MatchString(c.Regexp, line)
				if match {
					foundKeyword = true
					v.Results.Hits[strconv.Itoa(lineNumber)+":"+c.Prefix] = line
				} else if err != nil {
					log.Println("REGEXP ERRR:", err)
				}
			}
			// if v.Name == "main.go" {
			// 	// log.Println("searching:", c.Bytes, c.ByteSlice, lineBytes)
			// }
			if len(c.ByteSlice) > 0 && bytes.Contains(lineBytes, c.ByteSlice) {
				foundKeyword = true
				v.Results.Hits[strconv.Itoa(lineNumber)+":"+c.Prefix] = line
			}
			//KRISTINN: ADD NEW CHECKS HERE
		}
		lineNumber++
	}

	if foundKeyword {
		fileBufferMap[rand.Intn(len(fileBufferMap))] <- v
	} else {
		readyToUnlock = true
	}
}

func WalkDirectories(dir string) {
	_ = godirwalk.Walk(dir, &godirwalk.Options{
		Callback: func(osPathname string, info *godirwalk.Dirent) error {

			if !info.IsDir() {
				GlobalWaitGroup.Add(1)
				searchBufferMap[rand.Intn(len(searchBufferMap))] <- File{
					Name:  osPathname,
					IsDir: info.IsDir(),
					Results: SearchResults{
						Hits: make(map[string]string),
					},
				}
			}

			return nil
		},
		Unsorted: true,
	})

}
