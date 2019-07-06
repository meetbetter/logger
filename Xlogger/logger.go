package Xlogger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const TIMEFORMAT string = "2006-01-02"

//日志的level定义,ALL-OFF由低到高
const (
	ALL int = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
	OFF
)

//流量单位定义
const (
	_        = iota             //0
	KB int64 = 1 << (iota * 10) //1 << 10
	MB                          //1 << 20
	GB
	TB
)

//日志轮转的方式
const (
	NotRotate  int = iota //不论转日志文件
	DateRotate            //按日期轮转
	FileRotate            //按文件大小轮转
)

//logger的结构定义(日志对象的定义)
type Logger struct {
	//包级私有
	level        int      //日志的等级
	maxFileSize  int64    //日志的最大文件
	maxFileCount uint     //日志文件的最大个数
	rollDay      uint     //每隔多少天轮转日志
	rollWay      int      //日志轮转的方式
	console      bool     //是否控制台输出
	logObj       *LogFile //日志输出文件对象

	//TODO 协程池异步记录日志
}

//文件对象定义
type LogFile struct {
	dir        string        //文件目录
	fileName   string        //文件名字
	suffix     int           //文件后缀,用数字
	isCover    bool          //是否覆盖
	date       *time.Time    //时间
	mu         *sync.RWMutex //锁
	fileHandle *os.File      //文件句柄
	logger     *log.Logger   //go自带log包的logger对象
}

//logger的接口信息
type ILogger interface {
	SetConsole(b bool) error                                                        //设置是否控制台输出
	SetLevel(l int) error                                                           //设置输出级别
	SetRollFile(dir, name string, maxFileSize, maxFileCount uint, unit int64) error //按照文件大小轮转日志
	SetRollDate(dir, name string, interval uint)                                    //按照日期轮转日志
	Console(s ...interface{})                                                       //输出到控制台
	Debug(s ...interface{})                                                         //debug输出
	Info(s ...interface{})                                                          //info输出
	Warn(s ...interface{})                                                          //Warn输出
	Error(s ...interface{})                                                         //error输出
	Fatal(s ...interface{})                                                         //fatal输出
	catchPanic()                                                                    //捕获错误
}

//新建logger对象
//默认不轮转日志，控制台输出
func NewLogger() ILogger {
	return &Logger{
		level:   ERROR,
		rollWay: NotRotate,
		console: true,
		logObj:  nil,
	}

	//TODO 设置默认参数和文件，防止出错
}

//TODO 读取json配置

/*实现ILogger接口*/

//设置是否在控制台输出
func (this *Logger) SetConsole(b bool) error {
	if b != true && b != false {
		return errors.New("param error: not bool type")
	}

	this.console = b

	return nil
}

//设置输出级别
func (this *Logger) SetLevel(level int) error {
	if level < ALL || level > OFF {
		return errors.New("param error")
	}
	this.level = level

	return nil
}

//按照文件大小 轮转日志
func (this *Logger) SetRollFile(dir, name string, maxFileSize, maxFileCount uint, unit int64) error {
	if dir == "" || name == "" || maxFileSize == 0 || maxFileCount == 0 {
		return errors.New("SetRollFile param error ")
	}
	if this.rollWay != NotRotate {
		return errors.New("Logger rollWay status error ")
	}

	this.maxFileCount = maxFileCount
	this.maxFileSize = int64(maxFileSize) * unit
	this.rollWay = FileRotate
	this.logObj = &LogFile{
		dir:      dir,
		fileName: name,
		isCover:  false,
		mu:       new(sync.RWMutex),
	}

	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()

	if !isExist(dir) {
		err := os.Mkdir(dir, os.ModeDir|os.ModePerm)
		if err != nil {
			return errors.New("Mkdir error ")
		}
	}

	if !this.isMustRotate() {
		this.logObj.fileHandle, _ = os.OpenFile(dir+"/"+name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)

		if this.console == true { //是否输出到console
			this.logObj.logger = log.New(io.MultiWriter(this.logObj.fileHandle, os.Stderr), "\n", log.Ldate|log.Ltime|log.Lshortfile)
		} else {
			this.logObj.logger = log.New(this.logObj.fileHandle, "\n", log.Ldate|log.Ltime|log.Lshortfile)
		}

	} else {
		this.rotate()
	}
	//监测文件
	go this.fileMonitor()

	return nil
}

//按照时间轮转
// 目前固定每隔一天轮转一次 //TODO 修改限定时间
func (this *Logger) SetRollDate(dir, name string, interval uint) {
	if interval == 0 || dir == "" || name == "" {
		panic("Logger: SetRollData error!")
		return
	}
	if this.rollWay != NotRotate {
		return
	}
	this.rollWay = DateRotate
	this.rollDay = interval
	t, _ := time.Parse(TIMEFORMAT, time.Now().Add(24*time.Hour).Format(TIMEFORMAT))
	this.logObj = &LogFile{
		dir:      dir,
		fileName: name,
		isCover:  false,
		date:     &t,
		mu:       new(sync.RWMutex),
	}
	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()
	if !isExist(dir) {
		os.Mkdir(dir, os.ModeDir|os.ModePerm)
	}
	if !this.isMustRotate() {
		this.logObj.fileHandle, _ = os.OpenFile(dir+"/"+name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		//this.logObj.logger = log.New(this.logObj.fileHandle, "\n", log.Ldate|log.Ltime|log.Lshortfile)
		if this.console == true { //是否输出到console
			this.logObj.logger = log.New(io.MultiWriter(this.logObj.fileHandle, os.Stderr), "\n", log.Ldate|log.Ltime|log.Lshortfile)
		} else {
			this.logObj.logger = log.New(this.logObj.fileHandle, "\n", log.Ldate|log.Ltime|log.Lshortfile)
		}
	} else {
		this.rotate()
	}
	//监测文件
	go this.fileMonitor()
}

//Console level输出
func (this *Logger) Console(s ...interface{}) {
	if this.console {
		_, file, line, _ := runtime.Caller(2) //TODO 调用等级是否要修改
		short := file
		for i := len(file) - 1; i > 0; i-- { //截取调用本函数的文件名
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		this.logObj.logger.Println(file+":"+strconv.Itoa(line), s)
	}
}

//DEBUG level输出
func (this *Logger) Debug(s ...interface{}) {
	if this.level > DEBUG {
		return
	}
	defer this.catchPanic()
	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()
	this.logObj.logger.Output(2, fmt.Sprintln("debug:", s))
	this.Console("debug:", s)
}

//INFO level输出
func (this *Logger) Info(s ...interface{}) {
	if this.level > INFO {
		return
	}
	defer this.catchPanic()
	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()
	this.logObj.logger.Output(2, fmt.Sprintln("info:", s))
	this.Console("info:", s)
}

//WARN level输出
func (this *Logger) Warn(s ...interface{}) {
	if this.level > WARN {
		return
	}
	defer this.catchPanic()
	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()
	this.logObj.logger.Output(2, fmt.Sprintln("warn:", s))
	this.Console("warn:", s)
}

//ERROR level输出
func (this *Logger) Error(s ...interface{}) {
	if this.level > ERROR {
		return
	}
	defer this.catchPanic()
	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()
	this.logObj.logger.Output(2, fmt.Sprintln("error:", s))
	this.Console("error:", s)
}

//FATAL level输出
func (this *Logger) Fatal(s ...interface{}) {
	if this.level > FATAL {
		return
	}
	defer this.catchPanic()
	this.logObj.mu.Lock()
	defer this.logObj.mu.Unlock()
	this.logObj.logger.Output(2, fmt.Sprintln("fatal:", s))
	this.Console("fatal:", s)
}

//判断文件是否存在
func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err) //TODO os.IsExist()似乎有问题,改isNotExist()
}

//判断是否达到轮转条件
func (this *Logger) isMustRotate() bool {
	if this.rollWay == DateRotate {
		t, _ := time.Parse(TIMEFORMAT, time.Now().Format(TIMEFORMAT))
		if t.After(*this.logObj.date) {
			return true
		}

	} else if this.rollWay == FileRotate {
		if getFileSize(this.logObj.dir+"/"+this.logObj.fileName) >= this.maxFileSize {
			return true
		}
	}
	return false
}

//获取文件大小
func getFileSize(path string) int64 {
	f, e := os.Stat(path)
	if e != nil {
		return 0
	}
	return f.Size()
}

//具体的处理达到轮转条件时文件的轮转
func (this *Logger) rotate() {
	//如果是按日期轮转
	if this.rollWay == DateRotate {
		newname := this.logObj.dir + "/" + this.logObj.fileName + "." + this.logObj.date.Add(-24*time.Hour).Format(TIMEFORMAT) //-24表示命名为前一天的日志
		if !isExist(newname) {                                                                                                 //如果新生成的日志不存在
			if this.logObj.fileHandle != nil { //如果文件是打开状态
				this.logObj.fileHandle.Sync()
				this.logObj.fileHandle.Close()
			}
			if this.maxFileCount == 1 {
				err := os.Remove(this.logObj.dir + "/" + this.logObj.fileName)
				if err != nil {
					this.logObj.logger.Println("Logger: rotate error", err.Error())
				}
			} else {
				err := os.Rename(this.logObj.dir+"/"+this.logObj.fileName, newname)
				if err != nil {
					this.logObj.logger.Println("Logger: rotate error", err.Error())
				}
			}

			t, _ := time.Parse(TIMEFORMAT, time.Now().Format(TIMEFORMAT))
			this.logObj.date = &t
			this.logObj.fileHandle, _ = os.Create(this.logObj.dir + "/" + this.logObj.fileName)

			if this.console == true { //是否输出到console
				this.logObj.logger = log.New(io.MultiWriter(this.logObj.fileHandle, os.Stderr), "\n", log.Ldate|log.Ltime|log.Lshortfile)
			} else {
				this.logObj.logger = log.New(this.logObj.fileHandle, "\n", log.Ldate|log.Ltime|log.Lshortfile)
			}
		} else {
		}
	} else if this.rollWay == FileRotate {
		if this.maxFileCount == 1 {
			if this.logObj.fileHandle != nil {
				this.logObj.fileHandle.Sync()
				this.logObj.fileHandle.Close()
				os.Remove(this.logObj.dir + "/" + this.logObj.fileName)
			}
		} else {
			for i := this.maxFileCount; i >= 1; i-- {
				oldname := this.logObj.dir + "/" + this.logObj.fileName + "." + strconv.Itoa(int(i))
				if i == this.maxFileCount {
					if isExist(oldname) {
						os.Remove(oldname)
					}
				} else {
					if isExist(oldname) {
						os.Rename(oldname, this.logObj.dir+"/"+this.logObj.fileName+"."+strconv.Itoa(int(i+1)))
					}
				}
			}
			this.logObj.fileHandle.Sync()
			this.logObj.fileHandle.Close()
			os.Rename(this.logObj.dir+"/"+this.logObj.fileName, this.logObj.dir+"/"+this.logObj.fileName+"."+strconv.Itoa(1))
		}
		this.logObj.fileHandle, _ = os.Create(this.logObj.dir + "/" + this.logObj.fileName)

		if this.console == true { //是否输出到console
			this.logObj.logger = log.New(io.MultiWriter(this.logObj.fileHandle, os.Stderr), "\n", log.Ldate|log.Ltime|log.Lshortfile)
		} else {
			this.logObj.logger = log.New(this.logObj.fileHandle, "\n", log.Ldate|log.Ltime|log.Lshortfile)
		}
	}
}

//周期检测文件是否达到轮转条件
func (this *Logger) fileMonitor() {
	timer := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-timer.C:
			this.fileCheck()
		}
	}
}

//检查文件是否达到轮转条件的具体实现
func (this *Logger) fileCheck() {
	defer func() {
		if err := recover(); err != nil {
			this.logObj.logger.Println(err)
		}
	}()
	if this.logObj != nil && this.isMustRotate() {
		this.logObj.mu.Lock()
		defer this.logObj.mu.Unlock()
		this.rotate()
	}
}

//捕捉panic
func (this *Logger) catchPanic() {
	if err := recover(); err != nil {
		this.logObj.logger.Println("catchPanic:", err)
	}
}
