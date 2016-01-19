package main

// parser for cisco command output:
// show bgp vpnv4 unicast all neighbors

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

type neigh struct {
	addr        string
	vrf         string
	remoteAs    string
	state       string
	uptime      string
	prefixCount string
}

type neighScanner struct {
	table map[string]*neigh
	curr  *neigh
}

func main() {
	log.Printf("main: reading from stdin")

	linesFound := 0
	scanner := &neighScanner{table: map[string]*neigh{}}

	consume := func(line string, lineNumber int) error {
		linesFound++
		return lineParser(scanner, line, lineNumber)
	}

	if err := scanFile(os.Stdin, consume); err != nil {
		log.Printf("main: %v", err)
	}

	log.Printf("main: reading from stdin: done: %d lines", linesFound)

	log.Printf("main: found %d neighbors", len(scanner.table))

	fmt.Printf("%-15s %-14s %-6s %-11s %-7s %6s\n", "Neighbor", "VRF", "ASN", "State", "Uptime", "Prefixes")
	for _, n := range scanner.table {
		fmt.Printf("%-15s %-14s %-6s %-11s %-7s %6s\n", n.addr, n.vrf, n.remoteAs, n.state, n.uptime, n.prefixCount)
	}
}

//BGP neighbor is 1.1.1.1,  vrf VRFNAME,  remote AS 65000, external link
//  BGP state = Established, up for 5w2d
//  Session state = Established, up for 1y8w
//(...)
//    Prefixes Current:               0         26 (Consumes 2080 bytes)

func lineParser(scanner *neighScanner, line string, lineNum int) error {

	if strings.HasPrefix(line, "BGP neighbor is ") {

		f := strings.Fields(line)
		if len(f) < 4 {
			return fmt.Errorf("lineParser: short bgp neighbor line: line=%d [%s]", lineNum, line)
		}

		id := f[3][:len(f[3])-1]

		var vrf, asn string

		if f[4] == "vrf" {
			if len(f) < 9 {
				return fmt.Errorf("lineParser: bad bgp neighbor vrf line: line=%d [%s]", lineNum, line)
			}

			vrf = f[5][:len(f[5])-1]
			asn = f[8][:len(f[8])-1]
		} else {
			if len(f) < 7 {
				return fmt.Errorf("lineParser: bad bgp neighbor line: line=%d [%s]", lineNum, line)
			}
			vrf = "--"
			asn = f[6][:len(f[6])-1]
		}

		key := fmt.Sprintf("%s:%s", id, vrf)

		n, ok := scanner.table[key]
		if !ok {
			n = &neigh{addr: id}
			scanner.table[key] = n
		}

		n.vrf = vrf
		n.remoteAs = asn

		scanner.curr = n

		return nil
	}

	if strings.HasPrefix(line, "  BGP state = ") || strings.HasPrefix(line, "  Session state = ") {
		if scanner.curr == nil {
			return fmt.Errorf("lineParser: hit state without neighbor: line=%d [%s]", lineNum, line)
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			return fmt.Errorf("lineParser: bad bgp state line: line=%d [%s]", lineNum, line)
		}
		if len(f) < 7 {
			scanner.curr.state = f[3]
			scanner.curr.uptime = "?"
		} else {
			scanner.curr.state = f[3][:len(f[3])-1]
			scanner.curr.uptime = f[6]
		}
		return nil
	}

	if strings.HasPrefix(line, "    Prefixes Current:") {
		if scanner.curr == nil {
			return fmt.Errorf("lineParser: hit prefix count without neighbor: line=%d [%s]", lineNum, line)
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			return fmt.Errorf("lineParser: bad bgp prefixes line: line=%d [%s]", lineNum, line)
		}
		scanner.curr.prefixCount = f[3]
		return nil
	}

	return nil // no error
}

type lineConsumerFunc func(line string, lineNumber int) error

func scanFile(f *os.File, consumer lineConsumerFunc) error {
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var lastErr error
	i := 0

	for scanner.Scan() {
		i++
		line := scanner.Text()
		if err := consumer(line, i); err != nil {
			lastErr = fmt.Errorf("scanFile: error consuming line %d [%s]: %v", i, line, err)
			log.Printf("%v", lastErr)
			return lastErr
		}
	}

	if err := scanner.Err(); err != nil {
		lastErr = fmt.Errorf("scanFile: error scanning: %v", err)
	}

	return lastErr
}
