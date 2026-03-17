package linear

const issueFields = `
	id
	identifier
	title
	description
	priority
	state { name }
	branchName
	url
	labels { nodes { name } }
	inverseRelations(first: 50) {
		nodes {
			type
			issue { id }
		}
	}
	createdAt
	updatedAt
`

const queryIssuesByStates = `
query IssuesByStates($projectSlug: String!, $states: [String!]!) {
	issues(
		filter: {
			project: { slugId: { eq: $projectSlug } }
			state: { name: { in: $states } }
		}
		first: 100
	) {
		nodes {` + issueFields + `
		}
	}
}
`

const queryIssueStatesByIDs = `
query IssueStatesByIDs($ids: [ID!]!) {
	issues(
		filter: { id: { in: $ids } }
		first: 100
	) {
		nodes {
			id
			state { name }
		}
	}
}
`
