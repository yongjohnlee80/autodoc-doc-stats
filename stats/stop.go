package stats

import "strings"

// stop are the English stop words: the words every text uses, which say nothing of what this one
// is about. They are counted as words but never listed as frequent.
var stop = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`
		a about above after again against all also am an and any are aren't as at
		be because been before being below between both but by
		can can't cannot could couldn't
		did didn't do does doesn't doing don't down during
		each either else even ever every
		few for from further
		get gets got
		had hadn't has hasn't have haven't having he he'd he'll he's her here here's hers herself
		him himself his how how's however
		i i'd i'll i'm i've if in into is isn't it it's its itself
		just
		let's like
		may me might more most much must mustn't my myself
		neither no nor not now
		of off on once one only or other ought our ours ourselves out over own
		per
		rather
		same shall shan't she she'd she'll she's should shouldn't since so some still such
		than that that's the their theirs them themselves then there there's these they they'd
		they'll they're they've this those though through thus to too
		under until up upon us
		very via
		was wasn't we we'd we'll we're we've were weren't what what's when when's where where's
		whether which while who who's whom whose why why's will with within without won't would
		wouldn't
		yet you you'd you'll you're you've your yours yourself yourselves
	`) {
		m[w] = true
	}
	return m
}()
