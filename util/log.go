package util

import (
	"fmt"
	"io"
	"log"
	"os"
)

var (
	Debug *log.Logger
	Info  *log.Logger
	Warn  *log.Logger
	Error *log.Logger

	ShareLog *log.Logger
	BlockLog *log.Logger
)

func InitLog(infoFile, errorFile, shareFile, blockFile string) {
	log.Println("infoFile:", infoFile)
	log.Println("errorFile:", errorFile)
	log.Println("shareFile:", shareFile)
	log.Println("blockFile:", blockFile)
	infoFd, err := os.OpenFile(infoFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open info log file:", err)
	}

	errorFd, err := os.OpenFile(errorFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open error log file:", err)
	}

	shareFd, err := os.OpenFile(shareFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open share log file:", err)
	}

	blockFd, err := os.OpenFile(blockFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open block log file:", err)
	}

	Debug = log.New(io.MultiWriter(os.Stdout, infoFd), "[D] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	Info = log.New(io.MultiWriter(os.Stdout, infoFd), "[I] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	Warn = log.New(io.MultiWriter(os.Stderr, infoFd, errorFd), "[W] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	Error = log.New(io.MultiWriter(os.Stderr, infoFd, errorFd), "[E] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	ShareLog = log.New(io.MultiWriter(shareFd, os.Stdout), "[S]", log.Ldate|log.Lmicroseconds)
	BlockLog = log.New(io.MultiWriter(blockFd, os.Stdout), "[B]", log.Ldate|log.Lmicroseconds)
}

type Logger struct {
	l *log.Logger
}

func (l *Logger) Print(v ...interface{}) {
	_ = l.l.Output(2, fmt.Sprint(v...))
}

func (l *Logger) Println(v ...interface{}) {
	_ = l.l.Output(2, fmt.Sprintln(v...))
}

func (l *Logger) Printf(format string, v ...interface{}) {
	_ = l.l.Output(2, fmt.Sprintf(format, v...))
}

func (l *Logger) Fatal(v ...interface{}) {
	_ = l.l.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}

func (l *Logger) Fatalln(v ...interface{}) {
	_ = l.l.Output(2, fmt.Sprintln(v...))
	os.Exit(1)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	_ = l.l.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (l *Logger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	_ = l.l.Output(2, s)
	panic(s)
}

func (l *Logger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	_ = l.l.Output(2, s)
	panic(s)
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	_ = l.l.Output(2, s)
	panic(s)
}
