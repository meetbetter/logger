package main

import (
	"myProjects/Xlogger/Xlogger"
)

func main() {
	log1 := Xlogger.NewLogger()
	log1.SetLevel(Xlogger.DEBUG)                                 //设置输出级别，默认是ERROR
	log1.SetConsole(false)                                       //设置是否是控制台输出，默认是true
	log1.SetRollFile(`./logs`, `mylog.log1`, 15, 10, Xlogger.MB) //设置文件大小轮转
	log1.Console("2222Console :hahahha11111111aaaaaa")
	log1.Debug("222Debugcccccccccc hahahha22222222222")
	log1.Error("2222errorccccccccccccccc: hahhaha333333333333")

	logger_err := Xlogger.NewLogger()
	logger_err.SetLevel(Xlogger.DEBUG)                                       //设置输出级别，默认是ERROR
	logger_err.SetConsole(true)                                              //设置是否是控制台输出，默认是true
	logger_err.SetRollFile(`./logs`, `mylog_error.log1`, 15, 10, Xlogger.MB) //设置文件大小轮转
	logger_err.Error("222logger_errcccccccccccc4444444444444")
}
