//
// Copyright (c) 2015 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package utils

import (
	"github.com/lpabon/godbc"
	"log"
	"os"
)

type LogLevel int

const (
	LEVEL_NOLOG LogLevel = iota
	LEVEL_CRITICAL
	LEVEL_ERROR
	LEVEL_WARNING
	LEVEL_INFO
	LEVEL_DEBUG
)

type Logger struct {
	critlog, errorlog, infolog *log.Logger
	debuglog, warninglog       *log.Logger

	level LogLevel
}

func NewLogger(prefix string, level LogLevel) *Logger {
	godbc.Require(level >= 0, level)
	godbc.Require(level <= LEVEL_DEBUG, level)

	l := &Logger{}

	if level == LEVEL_NOLOG {
		l.level = LEVEL_DEBUG
	} else {
		l.level = level
	}

	l.critlog = log.New(os.Stderr, prefix+" CRITICAL ", log.Llongfile|log.LstdFlags)
	l.errorlog = log.New(os.Stderr, prefix+" ERROR ", log.Llongfile|log.LstdFlags)
	l.warninglog = log.New(os.Stderr, prefix+" WARNING ", log.LstdFlags)
	l.infolog = log.New(os.Stdout, prefix+" INFO ", log.LstdFlags)
	l.debuglog = log.New(os.Stdout, prefix+" DEBUG ", log.Llongfile|log.LstdFlags)

	godbc.Ensure(l.critlog != nil)
	godbc.Ensure(l.errorlog != nil)
	godbc.Ensure(l.warninglog != nil)
	godbc.Ensure(l.infolog != nil)
	godbc.Ensure(l.debuglog != nil)

	return l
}

func (l *Logger) Level() LogLevel {
	return l.level
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) Critical(format string, v ...interface{}) {
	if l.level >= LEVEL_CRITICAL {
		l.critlog.Printf(format, v...)
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.level >= LEVEL_ERROR {
		l.errorlog.Printf(format, v...)
	}
}

func (l *Logger) Err(err error) {
	if l.level >= LEVEL_ERROR {
		l.errorlog.Print(err)
	}
}

func (l *Logger) Warning(format string, v ...interface{}) {
	if l.level >= LEVEL_WARNING {
		l.warninglog.Printf(format, v...)
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.level >= LEVEL_INFO {
		l.infolog.Printf(format, v...)
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level >= LEVEL_DEBUG {
		l.debuglog.Printf(format, v...)
	}
}
