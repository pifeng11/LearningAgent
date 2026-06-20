package skills

type Registry struct {
	items map[string]Skill
}

func NewRegistry() *Registry {
	return &Registry{items: map[string]Skill{}}
}

func (r *Registry) Register(skill Skill) {
	r.items[skill.Name()] = skill
}

func (r *Registry) Get(name string) (Skill, bool) {
	skill, ok := r.items[name]
	return skill, ok
}
