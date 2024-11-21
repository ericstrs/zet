package zet

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/ericstrs/zet/internal/meta"
	"github.com/ericstrs/zet/internal/storage"

	"github.com/ollama/ollama/api"
)

const (
	system = `<system>
  You are an AI assistant integrated into a CLI Zettelkasten system. Your task is to analyze the
  content of two zettels where one contains a link to the other. Based on the content of both
  zettels, you need to generate a short, informative description explaining why the zettel that
	the link references is important and why the user should follow it. When creating the description
	for the link, it should be related to the content in the current zettel. Since the knowledge base
	belongs to a single person, use first person writing style.

  <instructions>
  1. The content of the current zettel (source zettel) will be provided.
  2. The content of the referenced zettel (target zettel) will also be provided.
	3. Generate a **concise, single-sentence** description explaining why the link to the target
  zettel is important given the context of the current note.
  </instructions>

  <format>
  * Analyze the content of the source zettel.
  * Analyze the content of the target zettel.
  * Generate a descriptive sentence explaining why the link to the target zettel is important.
  </format>

  <example>
	<input>
	<source>
	# What is a reverse proxy?

	In computer networks, a reverse proxy is a proxy server that appears to any client to be an ordinary web server, but in reality merely acts as an intermediary that forwards the client’s requests to one or more ordinary web servers. Reverse proxies help increase   scalability, performance, resilience, and security, but they also carry a number of risks.

	Companies that run web servers often set up reverse proxies to facilitate the communication between an internet user’s browser and the web server(s). An important advantage of dosing so is that the web servers can be hidden behind a firewall on a company-internal network, and only the reverse proxy needs to be directly exposed to the internet. Reverse proxy servers are implemented in popular open-source web servers, such as Apache, Nginx, and Caddy. Dedicated reverse proxy servers, such as the open source software HAProxy and Squit, are used by some of the largest websites on the Internet.

	A reverse proxy can track all IP addresses making requests through it and it can also read and modify any non-encrypted traffic and comes with the risk of logging passwords or injecting malware if compromised by a malicious party.

	Reverse proxies differ from forward proxies, which are used when the client is restricted to a private, internal network and asks a forward proxy to retrieve resources from the public internet.

  ## Attack areas

	* If reverse proxies aren’t update daily, then they could be subject to a zero day.

	See:

	* [20230127234254](../20230127234254) What is a proxy?
	</source>

  <target>
	# What is a proxy?

	An application-level gateway, also known as a proxy, acts as an intermediary between a client and a server that is providing a service. It takes data in and just forwards that data to some other intended place.

	There are several different types of proxies:

	1. Forward proxy:
		* Act as an intermediary for requests from clients seeking resources from other servers. They are commonly used to access restricted content, improve security, and manage traffic.
	1. Reverse proxy†r:
		* Sit in front of web servers and forward client requests to those servers
	1. Transparent proxy:
		* Intercepts and modifies requests without the client’s knowledge, often used by organizations for caching and content filtering.
	1. Cache proxy:
		* Stores frequently accessed resources to reduce the load on the target server.

	Some common use cases of proxies:

	* Web scraping:
		* Proxies can be used to scrape websites without revealing the IP address of the scraper.
	* Accessing geo-restricted content
	* Hiding IP addresses
	* Network security:
		* Proxies can be used to improve network security by filtering out malicious traffic.

	See:

	* [20230127210850](../20230127210850) What is a security gateway?

	[^r]: [20240809000017](../20240809000017) What is a reverse proxy?
  </target>
	</input>

	<output>
	Explains the fundamental concept of proxies and to see different types of proxies, including reverse proxies.
	</output>
	</example>

	Ensure your description is concise and directly related to the content of both zettels. Your output should only be your description.

  <input_format>
  Source Zettel Content: [Text of the source zettel]
	Target Zettel Content: [Text of the target zettel]
  </input_format>

  <output_format>
  [Your one sentence description here]
  </output_format>
	</system>`
)

var (
	linkRegex = regexp.MustCompile(`^.*(\[(.+)\]\(\.\./(.*?)/?\) (.+))`)
)

// AnnotateLink annotates zettel links with a reason why someone should
// follow the link to the referenced content
func AnnotateLink(s *storage.Storage, zetDir, dbPath, content string) ([]string, error) {
	var annotatedLinks []string
	cl, err := meta.CurrLink(zetDir)
	if err != nil {
		return nil, err
	}

	fromZettel, err := constructZettel(s, cl)
	if err != nil {
		return nil, fmt.Errorf("failed to construct from zettel string: %v\n", err)
	}

	links := meta.ParseLinks(content)
	for i := range links {
		zettel, err := constructZettel(s, links[i])
		if err != nil {
			continue
		}

		client, err := api.ClientFromEnvironment()
		if err != nil {
			log.Fatal(err)
		}

		messages := []api.Message{
			api.Message{
				Role:    "system",
				Content: system,
			},
			api.Message{
				Role: "user",
				// Note: Appending the reminder for the chatbots purpose is necessary
				// for instances involving big zettel's to give a single.
				// Otherwise, the chatbot will lose sight of the goal and do
				// things like refactor the text.
				Content: "<input>\n<source>\n" + fromZettel + "\n</source>\n<target>\n" + zettel + "\n</target>Remember, I want you to Generate a **concise, single-sentence** description explaining why the link to the target zettel is important. ONLY give this single sentence, NOTHING else. This single sentence should look something like: \"Follow this link to understand [brief description of content or reason for the topic].\"\n</input>",
			},
		}
		ctx := context.Background()
		req := &api.ChatRequest{
			Model:    "llama3.1",
			Messages: messages,
		}
		var response string
		respFunc := func(resp api.ChatResponse) error {
			response += resp.Message.Content
			return nil
		}
		err = client.Chat(ctx, req, respFunc)
		if err != nil {
			return nil, err
		}

		annotatedLinks = append(annotatedLinks, links[i]+"\n  * "+response)
	}

	return annotatedLinks, nil
}

// constructZettel returns a string representation of a storage.Zettel
// struct
func constructZettel(s *storage.Storage, link string) (string, error) {
	tx, err := s.DB.Beginx()
	if err != nil {
		return "", fmt.Errorf("Failed to create transaction: %v\n", err)
	}

	matches := linkRegex.FindStringSubmatch(link)
	if len(matches) <= 1 {
		return "", nil
	}

	iso := matches[2]
	id, err := storage.ZettelIdDir(tx, iso)
	if err != nil {
		return "", nil
	}
	z, err := storage.GetZettel(s.DB, id)
	if err != nil {
		return "", fmt.Errorf("Error retrieving sub-zettel: %v", err)
	}

	zettel := "# " + z.Title + "\n"
	if z.Body != "" {
		zettel += z.Body + "\n"
	}
	for _, l := range z.Links {
		zettel += "* " + l.Content + "\n"
	}
	if len(z.Tags) > 0 {
		var tags string
		for _, t := range z.Tags {
			tags += fmt.Sprintf("#%s ", t.Name)
		}
		zettel += "    " + tags + "\n"
	}
	return zettel, nil
}
