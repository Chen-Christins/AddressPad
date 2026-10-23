package sshconfig

import (
	"addresspad/internal/model"
	"fmt"
	"strings"
)

// Merge is side-effect free; callers persist the entire validated result once.
func Merge(config model.Config, servers []model.Server, overwrite bool) (model.Config, []string, error) {
	c := config.Clone()
	c.Normalize()
	indices := map[string]int{}
	for i, s := range c.Servers {
		indices[strings.ToLower(s.Name)] = i
	}
	var actions []string
	for _, s := range servers {
		key := strings.ToLower(s.Name)
		if i, exists := indices[key]; exists {
			if !overwrite {
				actions = append(actions, "skip "+s.Name+": already exists (use --overwrite)")
				continue
			}
			old := &c.Servers[i]
			old.Host, old.User, old.Port, old.IdentityFile, old.ProxyJump = s.Host, s.User, s.Port, s.IdentityFile, s.ProxyJump
			actions = append(actions, "update "+old.Name)
		} else {
			s.Normalize()
			indices[key] = len(c.Servers)
			c.Servers = append(c.Servers, s)
			actions = append(actions, fmt.Sprintf("import %s %s", s.Name, s.Host))
		}
	}
	if err := c.Validate(); err != nil {
		return model.Config{}, nil, err
	}
	return c, actions, nil
}
