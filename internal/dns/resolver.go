package dns

// import (
// 	"fmt"
// 	"log"
// 	"regexp"
// )

// var reResolvers = regexp.MustCompile(`^([^:]+):(\d+)/(tcp|udp)$`)

// type server struct {
// 	address string
// 	port    string
// 	proto   string
// }

// func parse(resolver string) (server, error) {
// 	matches := reResolvers.FindStringSubmatch(resolver)
// 	if len(matches) != 4 {
// 		return server{}, fmt.Errorf("failed to parse dns line: %s", resolver)
// 	}
// 	_proto := matches[3]
// 	if _proto != "udp" && _proto != "tcp" {
// 		return server{}, fmt.Errorf("fauled to parse proto it must be udp/tcp")
// 	}
// 	return server{
// 		address: matches[1],
// 		port:    matches[2],
// 		proto:   _proto,
// 	}, nil
// }

// func parseResolvers(resolvers []string) []server {
// 	var servers = []server{}
// 	for i, resolv := range resolvers {
// 		server, err := parse(resolv)
// 		if i == 0 && server.proto != "tcp" {
// 			log.Fatalln("first resolver must be with proto \"tcp\" not:", server.proto)
// 		}
// 		if err != nil {
// 			log.Println(err)
// 			continue
// 		}
// 		servers = append(servers, server)
// 	}
// 	return servers
// }
