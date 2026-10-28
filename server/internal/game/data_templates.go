package game

// DefaultBuildTemplates are the build templates every player starts with (each step = +1 level).
var DefaultBuildTemplates = []BuildTemplate{
	{
		ID:   "tpl_start",
		Name: "",
		Steps: []BuildingID{
			BWood, BClay, BIron, BWood, BClay, BMain, BWood, BClay, BIron, BStorage, BMain,
			BWood, BClay, BIron, BFarm, BWood, BClay, BIron, BStorage, BMain, BBarracks,
			BWood, BClay, BIron, BFarm, BStorage, BWood, BClay, BIron, BMain, BMain,
			BSmithy, BWood, BClay, BIron, BStorage, BFarm, BWood, BClay, BIron,
		},
	},
	{
		ID:   "tpl_def",
		Name: "",
		Steps: []BuildingID{BMain, BMain, BBarracks, BWall, BWall, BBarracks, BBarracks, BWall, BWall, BWall, BFarm,
			BStorage, BBarracks, BBarracks, BWall, BWall, BWall, BWall, BWall, BWall},
	},
	{
		ID:   "tpl_off",
		Name: "",
		Steps: []BuildingID{BMain, BMain, BMain, BMain, BBarracks, BBarracks, BBarracks, BBarracks, BSmithy, BSmithy,
			BSmithy, BSmithy, BSmithy, BStable, BStable, BStable, BFarm, BFarm, BStorage, BStorage},
	},
}

// CloneBuildTemplates deep-copies the defaults for a new human.
func CloneBuildTemplates() []BuildTemplate {
	out := make([]BuildTemplate, len(DefaultBuildTemplates))
	for i, t := range DefaultBuildTemplates {
		out[i] = BuildTemplate{ID: t.ID, Name: t.Name, Steps: append([]BuildingID(nil), t.Steps...)}
	}
	return out
}
