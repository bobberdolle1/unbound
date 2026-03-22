package engine

import (
	"bytes"
	"fmt"
	"strings"
)

type HTTPManipulation struct {
	HostCaseChange  bool
	SpaceInjection  bool
	MethodEOL       bool
	TabReplacement  bool
	MixedLineEnding bool
}

func ManipulateHTTPRequest(request []byte, manipulation HTTPManipulation) []byte {
	result := make([]byte, len(request))
	copy(result, request)

	if manipulation.HostCaseChange {
		result = changeHostCase(result)
	}

	if manipulation.SpaceInjection {
		result = injectSpaceAfterHost(result)
	}

	if manipulation.MethodEOL {
		result = addMethodEOL(result)
	}

	if manipulation.TabReplacement {
		result = replaceSpacesWithTabs(result)
	}

	if manipulation.MixedLineEnding {
		result = mixLineEndings(result)
	}

	return result
}

func changeHostCase(request []byte) []byte {
	hostPattern := []byte("Host:")
	hostPatternLower := []byte("host:")

	if idx := bytes.Index(request, hostPattern); idx != -1 {
		result := make([]byte, len(request))
		copy(result, request)
		copy(result[idx:idx+5], []byte("hOsT:"))
		return result
	}

	if idx := bytes.Index(request, hostPatternLower); idx != -1 {
		result := make([]byte, len(request))
		copy(result, request)
		copy(result[idx:idx+5], []byte("HoSt:"))
		return result
	}

	return request
}

func injectSpaceAfterHost(request []byte) []byte {
	hostPattern := []byte("Host: ")

	idx := bytes.Index(request, hostPattern)
	if idx == -1 {
		return request
	}

	result := make([]byte, 0, len(request)+1)
	result = append(result, request[:idx+5]...)
	result = append(result, ' ')
	result = append(result, request[idx+5:]...)

	return result
}

func addMethodEOL(request []byte) []byte {
	lines := bytes.Split(request, []byte("\r\n"))
	if len(lines) == 0 {
		return request
	}

	requestLine := string(lines[0])
	parts := strings.Split(requestLine, " ")

	if len(parts) < 3 {
		return request
	}

	parts[0] = parts[0] + " "

	lines[0] = []byte(strings.Join(parts, " "))

	return bytes.Join(lines, []byte("\r\n"))
}

func replaceSpacesWithTabs(request []byte) []byte {
	lines := bytes.Split(request, []byte("\r\n"))

	for i, line := range lines {
		if bytes.Contains(line, []byte("Host:")) {
			lines[i] = bytes.ReplaceAll(line, []byte(" "), []byte("\t"))
		}
	}

	return bytes.Join(lines, []byte("\r\n"))
}

func mixLineEndings(request []byte) []byte {
	result := bytes.ReplaceAll(request, []byte("\r\n"), []byte("\n"))

	lines := bytes.Split(result, []byte("\n"))

	for i := range lines {
		if i%2 == 0 {
			lines[i] = append(lines[i], '\r', '\n')
		} else {
			lines[i] = append(lines[i], '\n')
		}
	}

	return bytes.Join(lines, nil)
}

func GenerateHTTPVariants(baseRequest string) []string {
	variants := []string{baseRequest}

	manipulations := []HTTPManipulation{
		{HostCaseChange: true},
		{SpaceInjection: true},
		{MethodEOL: true},
		{TabReplacement: true},
		{HostCaseChange: true, SpaceInjection: true},
		{HostCaseChange: true, MethodEOL: true},
		{SpaceInjection: true, MethodEOL: true},
		{HostCaseChange: true, SpaceInjection: true, MethodEOL: true},
	}

	for _, manip := range manipulations {
		variant := ManipulateHTTPRequest([]byte(baseRequest), manip)
		variants = append(variants, string(variant))
	}

	return variants
}

type TCPManipulation struct {
	BadSum     bool
	BadSeq     bool
	DataNoACK  bool
	TCPMD5     bool
	LowTTL     int
	WindowSize int
}

func GetTCPManipulationArgs(manip TCPManipulation) []string {
	args := make([]string, 0)

	if manip.BadSum {
		args = append(args, "badsum")
	}

	if manip.BadSeq {
		args = append(args, "badseq")
	}

	if manip.DataNoACK {
		args = append(args, "datanoack")
	}

	if manip.TCPMD5 {
		args = append(args, "tcp_md5")
	}

	if manip.LowTTL > 0 {
		args = append(args, fmt.Sprintf("ttl=%d", manip.LowTTL))
	}

	if manip.WindowSize > 0 {
		args = append(args, fmt.Sprintf("wsize=%d", manip.WindowSize))
	}

	return args
}

type IPFragmentation struct {
	Mode       string
	FragSize   int
	FragOffset int
}

func GetIPFragmentationArgs(frag IPFragmentation) []string {
	args := make([]string, 0)

	switch frag.Mode {
	case "ipfrag1":
		args = append(args, "ipfrag1")
	case "ipfrag2":
		args = append(args, "ipfrag2")
	}

	if frag.FragSize > 0 {
		args = append(args, fmt.Sprintf("ipfrag_size=%d", frag.FragSize))
	}

	if frag.FragOffset > 0 {
		args = append(args, fmt.Sprintf("ipfrag_offset=%d", frag.FragOffset))
	}

	return args
}
