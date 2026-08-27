package authz

const ResourceProxy = "proxy"

var (
	ProxyRead           = Permission{Resource: ResourceProxy, Action: ActionRead}
	ProxyOperate        = Permission{Resource: ResourceProxy, Action: ActionOperate}
	ProxyWrite          = Permission{Resource: ResourceProxy, Action: ActionWrite}
	ProxySensitiveWrite = Permission{Resource: ResourceProxy, Action: ActionSensitiveWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceProxy,
		LabelKey: "Proxy Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read proxies",
				DescriptionKey: "View proxy lists and safe connection details.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionOperate,
				LabelKey:       "Test proxies",
				DescriptionKey: "Run proxy connectivity and quality checks.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Edit proxies",
				DescriptionKey: "Create, update, and change proxy status.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionSensitiveWrite,
				LabelKey:       "Manage proxy credentials",
				DescriptionKey: "Change proxy credentials or delete proxies.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}
