package dummytui

// This file seeds the five primary placeholder surfaces on the FINAL surface
// IDs (identity-manager/global-ssh/global-git/health/fixer per
// 02-UX-DIRECTION.md section 2) so that LaunchFrom bindings from keyless
// modal surfaces (create-flow, git-screen — added by later fan-out plans)
// resolve both against these placeholders AND against the real surface each
// one replaces via RegisterOrReplace. Fan-out surfaces REPLACE these entries
// in their OWN files — never edit this file to add a real surface.
func init() {
	Register(SurfaceDef{
		ID:            "identity-manager",
		Title:         "Identities",
		ActivationKey: "1",
		Screens: []ScreenDef{
			{ID: "entry", Render: func() string {
				return "Identity Manager — placeholder (replaced in Wave 4 by the real identity-manager surface)"
			}},
		},
	})
	Register(SurfaceDef{
		ID:            "global-ssh",
		Title:         "Global SSH",
		ActivationKey: "2",
		Screens: []ScreenDef{
			{ID: "entry", Render: func() string {
				return "Global SSH options — placeholder (replaced in Wave 4 by the real global-ssh surface)"
			}},
		},
	})
	Register(SurfaceDef{
		ID:            "global-git",
		Title:         "Global Git",
		ActivationKey: "3",
		Screens: []ScreenDef{
			{ID: "entry", Render: func() string {
				return "Global Git options — placeholder (replaced in Wave 4 by the real global-git surface)"
			}},
		},
	})
	Register(SurfaceDef{
		ID:            "health",
		Title:         "Health",
		ActivationKey: "4",
		Screens: []ScreenDef{
			{ID: "entry", Render: func() string {
				return "Health — placeholder (replaced in Wave 4 by the real health surface)"
			}},
		},
	})
	Register(SurfaceDef{
		ID:            "fixer",
		Title:         "Fixer",
		ActivationKey: "5",
		Screens: []ScreenDef{
			{ID: "entry", Render: func() string {
				return "Fixer — placeholder (replaced in Wave 4 by the real fixer surface)"
			}},
		},
	})
}
