package telemt

import "testing"

func FuzzKeyFromLink(f *testing.F) {
	f.Add("tg://proxy?server=example.com&port=443&secret=ee00112233445566778899aabbccddeeff6578616d706c652e636f6d")
	f.Add("not-a-link")
	f.Add("tg://proxy?secret=zz")
	f.Fuzz(func(t *testing.T, link string) {
		if len(link) > 4096 {
			return
		}
		_, _ = KeyFromLink(link)
	})
}
