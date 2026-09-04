package resources

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerGuides(server *mcp.Server) {
	guides := []struct {
		URI, Name, Title, Description, Content string
	}{
		{
			URI:         "jellyfin://guides/transcoding",
			Name:        "Transcoding Setup Guide",
			Title:       "Transcoding",
			Description: "Hardware transcoding configuration: Intel QSV, VAAPI, NVENC, Docker GPU passthrough",
			Content:     transcodingGuide,
		},
		{
			URI:         "jellyfin://guides/file-naming",
			Name:        "File Naming Conventions",
			Title:       "File Naming",
			Description: "Movie, TV, and music file naming standards for proper metadata matching",
			Content:     fileNamingGuide,
		},
		{
			URI:         "jellyfin://guides/remote-access",
			Name:        "Remote Access Setup",
			Title:       "Remote Access",
			Description: "Reverse proxy, VPN, and port forwarding options for accessing Jellyfin remotely",
			Content:     remoteAccessGuide,
		},
		{
			URI:         "jellyfin://guides/troubleshooting",
			Name:        "Troubleshooting Guide",
			Title:       "Troubleshooting",
			Description: "Common issues and solutions: library scans, metadata, playback, database recovery",
			Content:     troubleshootingGuide,
		},
		{
			URI:         "jellyfin://guides/library-setup",
			Name:        "Library Setup Guide",
			Title:       "Library Setup",
			Description: "Library organization, content separation, storage, and Docker volume configuration",
			Content:     librarySetupGuide,
		},
		{
			URI:         "jellyfin://guides/docker",
			Name:        "Docker Deployment Guide",
			Title:       "Docker",
			Description: "Docker volume mounts, GPU passthrough, permissions, compose files, and networking",
			Content:     dockerGuide,
		},
		{
			URI:         "jellyfin://guides/users-and-access",
			Name:        "Users and Access Control Guide",
			Title:       "Users & Access",
			Description: "User management, parental controls, per-library access, and invitation tools",
			Content:     usersAccessGuide,
		},
		{
			URI:         "jellyfin://guides/plugins",
			Name:        "Plugin Ecosystem Guide",
			Title:       "Plugins",
			Description: "Plugin repositories, recommended plugins, and configuration tips",
			Content:     pluginsGuide,
		},
		{
			URI:         "jellyfin://guides/migration",
			Name:        "Migration Guide",
			Title:       "Migration",
			Description: "Migrating from Plex or Emby to Jellyfin: library setup, metadata, plugins, and watch history",
			Content:     migrationGuide,
		},
		{
			URI:         "jellyfin://guides/performance",
			Name:        "Performance Tuning Guide",
			Title:       "Performance",
			Description: "Hardware transcoding optimization, database maintenance, cache sizing, and network tuning",
			Content:     performanceGuide,
		},
	}

	for _, g := range guides {
		guide := g // capture
		server.AddResource(&mcp.Resource{
			URI:         guide.URI,
			Name:        guide.Name,
			Title:       guide.Title,
			Description: guide.Description,
			MIMEType:    "text/markdown",
			Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant"}, Priority: 0.2},
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      guide.URI,
					MIMEType: "text/markdown",
					Text:     guide.Content,
				}},
			}, nil
		})
	}
}
