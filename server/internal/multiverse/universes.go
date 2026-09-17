package multiverse

import (
	"math/rand/v2"
)

// Category groups universes so that a single roll never offers three
// variations on the same idea. Diversity is what keeps results surprising.
type Category string

// The categories are chosen to pull in very different directions: some swap
// species, some swap era, some swap the physical material of reality itself.
const (
	CategoryMythic    Category = "mythic"
	CategoryCreature  Category = "creature"
	CategoryEra       Category = "era"
	CategoryArt       Category = "art"
	CategoryMaterial  Category = "material"
	CategoryScale     Category = "scale"
	CategoryObject    Category = "object"
	CategoryFuture    Category = "future"
	CategoryAbsurd    Category = "absurd"
	CategoryElemental Category = "elemental"
)

// Universe is one seed for a parallel world. The vision model receives a few
// of these and tailors the winner to the specific photo it is looking at.
type Universe struct {
	Name     string   `json:"name"`
	Category Category `json:"category"`
	// Hint tells the model what the subject becomes and what their world looks
	// like. It is deliberately short: the model does the scene-specific work.
	Hint string `json:"hint"`
}

// Library is the curated set of universes. Add freely; keep each hint vivid
// and keep categories balanced so random draws stay varied.
var Library = []Universe{
	// Mythic
	{"Infernal Court", CategoryMythic, "The subject is a horned demon noble in a cathedral of cooling lava; every object becomes a burning contract, bone relic or chained soul."},
	{"Drowned Pantheon", CategoryMythic, "The subject is a sea deity with barnacled skin in a sunken temple; objects become coral tablets, pearl instruments and shoals of fish."},
	{"Celestial Bureaucracy", CategoryMythic, "The subject is a jade-robed heavenly clerk on a cloud terrace; objects become scrolls of fate, ink brushes and floating seals."},
	{"Norse Twilight", CategoryMythic, "The subject is a rune-scarred frost giant beneath the ash tree at the end of days; objects become rune stones, horns and frozen weapons."},
	{"Wandering Yokai", CategoryMythic, "The subject is a fox spirit with lantern eyes in a rainy Edo alley; objects become paper talismans, masks and floating spirit lights."},

	// Creature
	{"Hive Mind", CategoryCreature, "The subject is a towering insectoid in a honeycomb city lit by amber; objects become chitin tools, wax tablets and larval companions."},
	{"Abyssal Angler", CategoryCreature, "The subject is a bioluminescent deep-sea creature in crushing darkness; objects become glowing lures, kelp and translucent jelly."},
	{"Mycelium Kin", CategoryCreature, "The subject is a fungal being with a shelf-mushroom crown in a spore-lit forest; objects become caps, gills and drifting spores."},
	{"Avian Aristocracy", CategoryCreature, "The subject is an elegant heron-headed noble in a marsh palace; objects become feather quills, egg vessels and reed instruments."},
	{"Silicon Reef", CategoryCreature, "The subject is a crystalline organism growing on a quartz plateau; objects become prisms, crystal lattices and refracted light."},

	// Era
	{"Pharaonic Dawn", CategoryEra, "The subject is a linen-clad scribe of ancient Egypt beside the Nile; objects become papyrus, reed pens and gilded amulets."},
	{"Silk Road Caravan", CategoryEra, "The subject is a spice merchant at a Samarkand oasis at dusk; objects become woven bags, brass lamps and camel tack."},
	{"Baroque Salon", CategoryEra, "The subject is a powdered aristocrat in a candlelit 1750s salon; objects become fans, snuff boxes and gilded letters."},
	{"Dust Bowl", CategoryEra, "The subject is a weathered farmer in 1930s Oklahoma under a blackened sky; objects become tin cups, seed sacks and worn tools."},
	{"Cold War Cosmonaut", CategoryEra, "The subject is a cosmonaut inside a cramped 1960s Soviet capsule; objects become analog dials, checklists and food tubes."},

	// Art
	{"Ukiyo-e Print", CategoryArt, "The subject is rendered as a Japanese woodblock print with flat colour, bold outlines and patterned waves; objects become fans, scrolls and lanterns."},
	{"Renaissance Fresco", CategoryArt, "The subject is a saint painted in cracked fresco on a chapel wall; objects become halos, chalices and illuminated books."},
	{"Soviet Poster", CategoryArt, "The subject is a heroic worker in a red-and-cream constructivist propaganda poster; objects become hammers, gears and banners."},
	{"Stop-Motion Clay", CategoryArt, "The subject is a hand-sculpted claymation character with visible fingerprints on a miniature set; objects become chunky clay props."},
	{"Medieval Marginalia", CategoryArt, "The subject is a creature drawn in the margin of an illuminated manuscript with gold leaf; objects become vines, snails and tiny swords."},

	// Material
	{"Folded Paper", CategoryMaterial, "The subject and the whole world are origami: creased paper with visible folds; objects become paper cranes, boxes and lanterns."},
	{"Blown Glass", CategoryMaterial, "The subject is made of translucent coloured glass in a glass-blower's furnace room; objects become bottles, marbles and vials."},
	{"Living Topiary", CategoryMaterial, "The subject is a figure of clipped hedge and moss in a formal garden; objects become flowers, seed pods and vines."},
	{"Cast Bronze", CategoryMaterial, "The subject is a patinated bronze statue in a rain-washed plaza; objects become bronze scrolls, spheres and laurel wreaths."},
	{"Molten Neon", CategoryMaterial, "The subject is a figure made of bent neon tubing in a dark workshop; objects become glowing signs, transformers and wire."},

	// Scale
	{"Colossus", CategoryScale, "The subject is a mountain-sized giant and the surroundings are a tiny city at their feet; objects become entire buildings, bridges or ships."},
	{"Microcosm", CategoryScale, "The subject is a microscopic organism among cells and dust motes; objects become organelles, pollen grains and salt crystals."},
	{"Dollhouse", CategoryScale, "The subject is a small doll in an oversized human living room; objects become thimbles, buttons and matchboxes."},
	{"Orbital", CategoryScale, "The subject is a planet-scale being in deep space; objects become moons, comets and rings of ice."},

	// Object
	{"Appliance Soul", CategoryObject, "The subject is a vintage household appliance with a face, in a kitchen from a forgotten decade; objects become plugs, dials and manuals."},
	{"Street Furniture", CategoryObject, "The subject is a sentient lamppost or bench in an empty midnight city; objects become tickets, coins and pigeons."},
	{"Musical Instrument", CategoryObject, "The subject is an instrument come to life in an abandoned concert hall; objects become sheet music, bows and tuning forks."},
	{"Clockwork Automaton", CategoryObject, "The subject is a brass automaton with exposed gears in a watchmaker's attic; objects become keys, springs and dials."},

	// Future
	{"Chrome Monastery", CategoryFuture, "The subject is a robotic monk in a minimalist white monastery orbiting a gas giant; objects become data crystals and prayer drones."},
	{"Solarpunk Commons", CategoryFuture, "The subject is a botanist-engineer in a lush vertical garden city; objects become seed drones, living tools and glass planters."},
	{"Post-Human Archive", CategoryFuture, "The subject is a translucent digital ghost in an infinite server hall; objects become floating holograms and data shards."},
	{"Terraform Crew", CategoryFuture, "The subject is a suited settler on a red Martian ridge at sunrise; objects become sample cases, tablets and oxygen lines."},

	// Absurd
	{"Vegetable Kingdom", CategoryAbsurd, "The subject is an anthropomorphic vegetable in a farmers' market metropolis; objects become leaves, seeds and price tags."},
	{"Cloud Shepherd", CategoryAbsurd, "The subject is a cloud with limbs herding smaller clouds above a patchwork countryside; objects become rainbows, kites and rain."},
	{"Balloon Parade", CategoryAbsurd, "The subject is an inflated latex balloon person at a floating parade; objects become strings, confetti and pumps."},
	{"Chessboard Realm", CategoryAbsurd, "The subject is a living chess piece on an endless checkered plain; objects become pawns, crowns and hourglasses."},

	// Elemental
	{"Ember Spirit", CategoryElemental, "The subject is a figure of smouldering embers and smoke in a burnt forest; objects become charcoal, sparks and ash."},
	{"Tidal Form", CategoryElemental, "The subject is a body of moving seawater on a black-sand shore; objects become shells, foam and driftwood."},
	{"Storm Wraith", CategoryElemental, "The subject is a figure of lightning and cloud above a wind-flattened plain; objects become kites, weather vanes and rain."},
	{"Frost Sentinel", CategoryElemental, "The subject is a being of blue ice in an aurora-lit tundra; objects become icicles, frozen lanterns and snow hares."},
}

// PickCandidates draws n universes from distinct categories using the given
// random source. Callers pass their own source so tests are deterministic.
func PickCandidates(r *rand.Rand, n int) []Universe {
	if n <= 0 {
		return nil
	}
	byCategory := make(map[Category][]Universe)
	var order []Category
	for _, u := range Library {
		if _, seen := byCategory[u.Category]; !seen {
			order = append(order, u.Category)
		}
		byCategory[u.Category] = append(byCategory[u.Category], u)
	}

	r.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	if n > len(order) {
		n = len(order)
	}

	picks := make([]Universe, 0, n)
	for _, c := range order[:n] {
		pool := byCategory[c]
		picks = append(picks, pool[r.IntN(len(pool))])
	}
	return picks
}
