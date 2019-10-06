package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/yzzyx/mr/config"
	"github.com/yzzyx/mr/imap"
	"github.com/yzzyx/mr/models"
	"github.com/yzzyx/mr/notmuch"
	"github.com/yzzyx/mr/ui"
	"gopkg.in/yaml.v2"
)

func indexAllFiles(db *notmuch.Database, lastRuntime time.Time, dirpath string) error {
	fd, err := os.Open(dirpath)
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	var entries []os.FileInfo
	for {
		entries, err = fd.Readdir(5)
		if err != nil && err != io.EOF {
			return err
		}

		if len(entries) == 0 {
			break
		}

		for k := range entries {
			name := entries[k].Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			newPath := filepath.Join(dirpath, name)
			if entries[k].IsDir() {
				err = indexAllFiles(db, lastRuntime, newPath)
				if err != nil {
					return err
				}
			} else if entries[k].ModTime().After(lastRuntime) {
				m, st := db.AddMessage(newPath)
				// We've already seen this one
				if st == notmuch.STATUS_DUPLICATE_MESSAGE_ID {
					continue
				}
				if st != notmuch.STATUS_SUCCESS {
					return errors.New(st.String())
				}
				fmt.Println(newPath)
				m.Destroy()
			}
		}
	}
	return nil
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func parsePathSetting(inPath string) string {
	if strings.HasPrefix(inPath, "$HOME") {
		inPath = userHomeDir() + inPath[5:]
	} else if strings.HasPrefix(inPath, "~/") {
		inPath = userHomeDir() + inPath[1:]
	}

	if strings.HasPrefix(inPath, "$") {
		end := strings.Index(inPath, string(os.PathSeparator))
		inPath = os.Getenv(inPath[1:end]) + inPath[end:]
	}
	if filepath.IsAbs(inPath) {
		return filepath.Clean(inPath)
	}

	p, err := filepath.Abs(inPath)
	if err == nil {
		return filepath.Clean(p)
	}
	return ""
}

func main() {

	var db *notmuch.Database
	var status notmuch.Status
	configPath := filepath.Join(userHomeDir(), ".config", "mr")

	cfgdata, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		fmt.Printf("Cannot read config file '%s': %s\n", configPath, err)
		os.Exit(1)
	}

	cfg := config.Config{}
	err = yaml.Unmarshal(cfgdata, &cfg)
	if err != nil {
		fmt.Printf("Cannot parse config file '%s': %s\n", configPath, err)
		os.Exit(1)
	}

	if cfg.Maildir == "" {
		cfg.Maildir = "~/.mail"
	}

	maildirPath := parsePathSetting(cfg.Maildir)

	// Create maildir if it doesnt exist
	err = os.MkdirAll(maildirPath, 0700)
	if err != nil {
		panic(err)
	}

	db, status = notmuch.OpenDatabase(maildirPath, notmuch.DATABASE_MODE_READ_WRITE)
	if status != notmuch.STATUS_SUCCESS {
		fmt.Println("Creating database...")
		db, status = notmuch.NewDatabase(maildirPath)
		if status != notmuch.STATUS_SUCCESS {
			fmt.Printf("Could not create database: error %s\n", status)
			return
		}
	}
	defer db.Close()

	if db.NeedsUpgrade() {
		fmt.Println("Database needs an upgrade - not implemented")
		return
	}

	//ts := time.Time{}
	//lastIndexedPath := filepath.Join(configPath, "lastindexed")
	//data, err := ioutil.ReadFile(lastIndexedPath)
	//if err == nil {
	//	err = json.Unmarshal(data, &ts)
	//	if err != nil {
	//		fmt.Println("Cannot unmarshal last index date:", err)
	//		return
	//	}
	//}

	//now := time.Now()

	// FIXME - Wrap this in a command
	// Reindex all files
	//fmt.Println("Indexing mailfiles...")
	//err = indexAllFiles(db, ts, maildirPath)
	//if err != nil {
	//	fmt.Println("Could not index maildir:", err)
	//	return
	//}

	//data, err = json.Marshal(now)
	//if err == nil {
	//	err = ioutil.WriteFile(lastIndexedPath, data, 0600)
	//	if err != nil {
	//		fmt.Println("Could not update last indexed timestamp:", err)
	//		return
	//	}
	//}

	//if h.cfg.IndexedMailDir == false {
	//	err = indexAllFiles(db, time.Time{}, h.maildirPath)
	//	if err != nil {
	//		return nil, err
	//	}
	//}
	// Create a IMAP setup for each mailbox
	for name, mailbox := range cfg.Mailboxes {
		folderPath := filepath.Join(maildirPath, name)
		err = os.MkdirAll(folderPath, 0700)
		if err != nil {
			panic(err)
		}

		h, err := imap.New(db, folderPath, mailbox)
		if err != nil {
			log.Fatal(err)
		}
		defer h.Close()

		//err = h.CheckMessages()
		//if err != nil {
		//	log.Fatal(err)
		//}
	}

	err = models.Setup(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot setup models:", err)
		return
	}

	err = ui.Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error in ui:", err)
		return
	}
	return

	q := db.CreateQuery("subject:sista")
	msgs := q.SearchMessages()
	for msgs.Valid() {
		m := msgs.Get()
		fmt.Println("matching: ", m.GetFileName())
		m.Destroy()
		msgs.MoveToNext()
	}
	msgs.Destroy()

	threads := q.SearchThreads()
	for threads.Valid() {
		t := threads.Get()
		fmt.Println("ThreadID:", t.GetThreadID())
		fmt.Println(" Newest:", t.GetNewestDate())
		fmt.Println(" Oldest:", t.GetOldestDate())
		fmt.Println(" Subject:", t.GetSubject())
		fmt.Println(" Author:", t.GetAuthors())

		msgs := t.GetMessages()
		for msgs.Valid() {
			m := msgs.Get()
			m.Destroy()
			fmt.Println("  Message:", m.GetFileName())
			msgs.MoveToNext()
		}
		msgs.Destroy()
		t.Destroy()
		threads.MoveToNext()
	}
	threads.Destroy()
	q.Destroy()
	fmt.Println(err)
}
