// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suffixarray

func sais_32(text []int32, textMax int, sa, tmp []int32) {
	if len(sa) != len(text) || len(tmp) < int(textMax) {
		panic("suffixarray: misuse of sais_32")
	}

	// Trivial base cases. Sorting 0 or 1 things is easy.
	if len(text) == 0 {
		return
	}
	if len(text) == 1 {
		sa[0] = 0
		return
	}

	// Establish slices indexed by text character
	// holding character frequency and bucket-sort offsets.
	// If there's only enough tmp for one slice,
	// we make it the bucket offsets and recompute
	// the character frequency each time we need it.
	var freq, bucket []int32
	if len(tmp) >= 2*textMax {
		freq, bucket = tmp[:textMax], tmp[textMax:2*textMax]
		freq[0] = -1 // mark as uninitialized
	} else {
		freq, bucket = nil, tmp[:textMax]
	}

	// The SAIS algorithm.
	// Each of these calls makes one scan through sa.
	// See the individual functions for documentation
	// about each's role in the algorithm.
	numLMS := placeLMS_32(text, sa, freq, bucket)
	if numLMS <= 1 {
		// 0 or 1 items are already sorted. Do nothing.
	} else {
		induceSubL_32(text, sa, freq, bucket)
		induceSubS_32(text, sa, freq, bucket)
		length_32(text, sa, numLMS)
		maxID := assignID_32(text, sa, numLMS)
		if maxID < numLMS {
			map_32(sa, numLMS)
			recurse_32(sa, tmp, numLMS, maxID)
			unmap_32(text, sa, numLMS)
		} else {
			// If maxID == numLMS, then each LMS-substring
			// is unique, so the relative ordering of two LMS-suffixes
			// is determined by just the leading LMS-substring.
			// That is, the LMS-suffix sort order matches the
			// (simpler) LMS-substring sort order.
			// Copy the original LMS-substring order into the
			// suffix array destination.
			copy(sa, sa[len(sa)-numLMS:])
		}
		expand_32(text, freq, bucket, sa, numLMS)
	}
	induceL_32(text, sa, freq, bucket)
	induceS_32(text, sa, freq, bucket)

	// Mark for caller that we overwrote tmp.
	tmp[0] = -1
}

func freq_32(text []int32, freq, bucket []int32) []int32 {
	if freq != nil && freq[0] >= 0 {
		return freq // already computed
	}
	if freq == nil {
		freq = bucket
	}

	for i := range freq {
		freq[i] = 0
	}
	for _, c := range text {
		freq[c]++
	}
	return freq
}

func bucketMin_32(text []int32, freq, bucket []int32) {
	freq = freq_32(text, freq, bucket)
	total := int32(0)
	for i, n := range freq {
		bucket[i] = total
		total += n
	}
}

func bucketMax_32(text []int32, freq, bucket []int32) {
	freq = freq_32(text, freq, bucket)
	total := int32(0)
	for i, n := range freq {
		total += n
		bucket[i] = total
	}
}

func placeLMS_32(text []int32, sa, freq, bucket []int32) int {
	bucketMax_32(text, freq, bucket)

	numLMS := 0
	lastB := int32(-1)

	// The next stanza of code (until the blank line) loop backward
	// over text, stopping to execute a code body at each position i
	// such that text[i] is an L-character and text[i+1] is an S-character.
	// That is, i+1 is the position of the start of an LMS-substring.
	// These could be hoisted out into a function with a callback,
	// but at a significant speed cost. Instead, we just write these
	// seven lines a few times in this source file. The copies below
	// refer back to the pattern established by this original as the
	// "LMS-substring iterator".
	//
	// In every scan through the text, c0, c1 are successive characters of text.
	// In this backward scan, c0 == text[i] and c1 == text[i+1].
	// By scanning backward, we can keep track of whether the current
	// position is type-S or type-L according to the usual definition:
	//
	//	- position len(text) is type S with text[len(text)] == -1 (the sentinel)
	//	- position i is type S if text[i] < text[i+1], or if text[i] == text[i+1] && i+1 is type S.
	//	- position i is type L if text[i] > text[i+1], or if text[i] == text[i+1] && i+1 is type L.
	//
	// The backward scan lets us maintain the current type,
	// update it when we see c0 != c1, and otherwise leave it alone.
	// We want to identify all S positions with a preceding L.
	// Position len(text) is one such position by definition, but we have
	// nowhere to write it down, so we eliminate it by untruthfully
	// setting isTypeS = false at the start of the loop.
	c0, c1, isTypeS := int32(0), int32(0), false
	for i := len(text) - 1; i >= 0; i-- {
		c0, c1 = text[i], c0
		if c0 < c1 {
			isTypeS = true
		} else if c0 > c1 && isTypeS {
			isTypeS = false

			// Bucket the index i+1 for the start of an LMS-substring.
			b := bucket[c1] - 1
			bucket[c1] = b
			sa[b] = int32(i + 1)
			lastB = b
			numLMS++
		}
	}

	// We recorded the LMS-substring starts but really want the ends.
	// Luckily, with two differences, the start indexes and the end indexes are the same.
	// The first difference is that the rightmost LMS-substring's end index is len(text),
	// so the caller must pretend that sa[-1] == len(text), as noted above.
	// The second difference is that the first leftmost LMS-substring start index
	// does not end an earlier LMS-substring, so as an optimization we can omit
	// that leftmost LMS-substring start index (the last one we wrote).
	//
	// Exception: if numLMS <= 1, the caller is not going to bother with
	// the recursion at all and will treat the result as containing LMS-substring starts.
	// In that case, we don't remove the final entry.
	if numLMS > 1 {
		sa[lastB] = 0
	}
	return numLMS
}

func induceSubL_32(text []int32, sa, freq, bucket []int32) {
	// Initialize positions for left side of character buckets.
	bucketMin_32(text, freq, bucket)

	// As we scan the array left-to-right, each sa[i] = j > 0 is a correctly
	// sorted suffix array entry (for text[j:]) for which we know that j-1 is type L.
	// Because j-1 is type L, inserting it into sa now will sort it correctly.
	// But we want to distinguish a j-1 with j-2 of type L from type S.
	// We can process the former but want to leave the latter for the caller.
	// We record the difference by negating j-1 if it is preceded by type S.
	// Either way, the insertion (into the text[j-1] bucket) is guaranteed to
	// happen at sa[i´] for some i´ > i, that is, in the portion of sa we have
	// yet to scan. A single pass therefore sees indexes j, j-1, j-2, j-3,
	// and so on, in sorted but not necessarily adjacent order, until it finds
	// one preceded by an index of type S, at which point it must stop.
	//
	// As we scan through the array, we clear the worked entries (sa[i] > 0) to zero,
	// and we flip sa[i] < 0 to -sa[i], so that the loop finishes with sa containing
	// only the indexes of the leftmost L-type indexes for each LMS-substring.
	//
	// The suffix array sa therefore serves simultaneously as input, output,
	// and a miraculously well-tailored work queue.

	// placeLMS_32 left out the implicit entry sa[-1] == len(text),
	// corresponding to the identified type-L index len(text)-1.
	// Process it before the left-to-right scan of sa proper.
	// See body in loop for commentary.
	k := len(text) - 1
	c0, c1 := text[k-1], text[k]
	if c0 < c1 {
		k = -k
	}

	// Cache recently used bucket index:
	// we're processing suffixes in sorted order
	// and accessing buckets indexed by the
	// int32 before the sorted order, which still
	// has very good locality.
	// Invariant: b is cached, possibly dirty copy of bucket[cB].
	cB := c1
	b := bucket[cB]
	sa[b] = int32(k)
	b++

	for i := 0; i < len(sa); i++ {
		j := int(sa[i])
		if j == 0 {
			// Skip empty entry.
			continue
		}
		if j < 0 {
			// Leave discovered type-S index for caller.
			sa[i] = int32(-j)
			continue
		}
		sa[i] = 0

		// Index j was on work queue, meaning k := j-1 is L-type,
		// so we can now place k correctly into sa.
		// If k-1 is L-type, queue k for processing later in this loop.
		// If k-1 is S-type (text[k-1] < text[k]), queue -k to save for the caller.
		k := j - 1
		c0, c1 := text[k-1], text[k]
		if c0 < c1 {
			k = -k
		}

		if cB != c1 {
			bucket[cB] = b
			cB = c1
			b = bucket[cB]
		}
		sa[b] = int32(k)
		b++
	}
}

func induceSubS_32(text []int32, sa, freq, bucket []int32) {
	// Initialize positions for right side of character buckets.
	bucketMax_32(text, freq, bucket)

	// Analogous to induceSubL_32 above,
	// as we scan the array right-to-left, each sa[i] = j > 0 is a correctly
	// sorted suffix array entry (for text[j:]) for which we know that j-1 is type S.
	// Because j-1 is type S, inserting it into sa now will sort it correctly.
	// But we want to distinguish a j-1 with j-2 of type S from type L.
	// We can process the former but want to leave the latter for the caller.
	// We record the difference by negating j-1 if it is preceded by type L.
	// Either way, the insertion (into the text[j-1] bucket) is guaranteed to
	// happen at sa[i´] for some i´ < i, that is, in the portion of sa we have
	// yet to scan. A single pass therefore sees indexes j, j-1, j-2, j-3,
	// and so on, in sorted but not necessarily adjacent order, until it finds
	// one preceded by an index of type L, at which point it must stop.
	// That index (preceded by one of type L) is an LMS-substring start.
	//
	// As we scan through the array, we clear the worked entries (sa[i] > 0) to zero,
	// and we flip sa[i] < 0 to -sa[i] and compact into the top of sa,
	// so that the loop finishes with the top of sa containing exactly
	// the LMS-substring start indexes, sorted by LMS-substring.

	// Cache recently used bucket index:
	cB := int32(0)
	b := bucket[cB]

	top := len(sa)
	for i := len(sa) - 1; i >= 0; i-- {
		j := int(sa[i])
		if j == 0 {
			// Skip empty entry.
			continue
		}
		sa[i] = 0
		if j < 0 {
			// Leave discovered LMS-substring start index for caller.
			top--
			sa[top] = int32(-j)
			continue
		}

		// Index j was on work queue, meaning k := j-1 is S-type,
		// so we can now place k correctly into sa.
		// If k-1 is S-type, queue k for processing later in this loop.
		// If k-1 is L-type (text[k-1] > text[k]), queue -k to save for the caller.
		k := j - 1
		c1 := text[k]
		c0 := text[k-1]
		if c0 > c1 {
			k = -k
		}

		if cB != c1 {
			bucket[cB] = b
			cB = c1
			b = bucket[cB]
		}
		b--
		sa[b] = int32(k)
	}
}

func length_32(text []int32, sa []int32, numLMS int) {
	end := 0 // index of current LMS-substring end (0 indicates final LMS-substring)

	// The encoding of N text int32s into a “length” word
	// adds 1 to each int32, packs them into the bottom
	// N*8 bits of a word, and then bitwise inverts the result.
	// That is, the text sequence A B C (hex 41 42 43)
	// encodes as ^uint32(0x42_43_44).
	// LMS-substrings can never start or end with 0xFF.
	// Adding 1 ensures the encoded int32 sequence never
	// starts or ends with 0x00, so that present int32s can be
	// distinguished from zero-padding in the top bits,
	// so the length need not be separately encoded.
	// Inverting the int32s increases the chance that a
	// 4-int32 encoding will still be ≥ len(text).
	// In particular, if the first int32 is ASCII (<= 0x7E, so +1 <= 0x7F)
	// then the high bit of the inversion will be set,
	// making it clearly not a valid length (it would be a negative one).
	//
	// cx holds the pre-inverted encoding (the packed incremented int32s).

	// This stanza (until the blank line) is the "LMS-substring iterator",
	// described in placeLMS_32 above, with one line added to maintain cx.
	c0, c1, isTypeS := int32(0), int32(0), false
	for i := len(text) - 1; i >= 0; i-- {
		c0, c1 = text[i], c0
		if c0 < c1 {
			isTypeS = true
		} else if c0 > c1 && isTypeS {
			isTypeS = false

			// Index j = i+1 is the start of an LMS-substring.
			// Compute length or encoded text to store in sa[j/2].
			j := i + 1
			var code int32
			if end == 0 {
				code = 0
			} else {
				code = int32(end - j)
			}
			sa[j>>1] = code
			end = j + 1
		}
	}
}

func assignID_32(text []int32, sa []int32, numLMS int) int {
	id := 0
	lastLen := int32(-1) // impossible
	lastPos := int32(0)
	for _, j := range sa[len(sa)-numLMS:] {
		// Is the LMS-substring at index j new, or is it the same as the last one we saw?
		n := sa[j/2]
		if n != lastLen {
			goto New
		}
		if uint32(n) >= uint32(len(text)) {
			// “Length” is really encoded full text, and they match.
			goto Same
		}
		{
			// Compare actual texts.
			n := int(n)
			this := text[j:][:n]
			last := text[lastPos:][:n]
			for i := 0; i < n; i++ {
				if this[i] != last[i] {
					goto New
				}
			}
			goto Same
		}
	New:
		id++
		lastPos = j
		lastLen = n
	Same:
		sa[j/2] = int32(id)
	}
	return id
}

func unmap_32(text []int32, sa []int32, numLMS int) {
	unmap := sa[len(sa)-numLMS:]
	j := len(unmap)

	// "LMS-substring iterator" (see placeLMS_32 above).
	c0, c1, isTypeS := int32(0), int32(0), false
	for i := len(text) - 1; i >= 0; i-- {
		c0, c1 = text[i], c0
		if c0 < c1 {
			isTypeS = true
		} else if c0 > c1 && isTypeS {
			isTypeS = false

			// Populate inverse map.
			j--
			unmap[j] = int32(i + 1)
		}
	}

	// Apply inverse map to subproblem suffix array.
	sa = sa[:numLMS]
	for i := 0; i < len(sa); i++ {
		sa[i] = unmap[sa[i]]
	}
}

func expand_32(text []int32, freq, bucket, sa []int32, numLMS int) {
	bucketMax_32(text, freq, bucket)

	// Loop backward through sa, always tracking
	// the next index to populate from sa[:numLMS].
	// When we get to one, populate it.
	// Zero the rest of the slots; they have dead values in them.
	x := numLMS - 1
	saX := sa[x]
	c := text[saX]
	b := bucket[c] - 1
	bucket[c] = b

	for i := len(sa) - 1; i >= 0; i-- {
		if i != int(b) {
			sa[i] = 0
			continue
		}
		sa[i] = saX

		// Load next entry to put down (if any).
		if x > 0 {
			x--
			saX = sa[x] // TODO bounds check
			c = text[saX]
			b = bucket[c] - 1
			bucket[c] = b
		}
	}
}

func induceL_32(text []int32, sa, freq, bucket []int32) {
	// Initialize positions for left side of character buckets.
	bucketMin_32(text, freq, bucket)

	// This scan is similar to the one in induceSubL_32 above.
	// That one arranges to clear all but the leftmost L-type indexes.
	// This scan leaves all the L-type indexes and the original S-type
	// indexes, but it negates the positive leftmost L-type indexes
	// (the ones that induceS_32 needs to process).

	// expand_32 left out the implicit entry sa[-1] == len(text),
	// corresponding to the identified type-L index len(text)-1.
	// Process it before the left-to-right scan of sa proper.
	// See body in loop for commentary.
	k := len(text) - 1
	c0, c1 := text[k-1], text[k]
	if c0 < c1 {
		k = -k
	}

	// Cache recently used bucket index.
	cB := c1
	b := bucket[cB]
	sa[b] = int32(k)
	b++

	for i := 0; i < len(sa); i++ {
		j := int(sa[i])
		if j <= 0 {
			// Skip empty or negated entry (including negated zero).
			continue
		}

		// Index j was on work queue, meaning k := j-1 is L-type,
		// so we can now place k correctly into sa.
		// If k-1 is L-type, queue k for processing later in this loop.
		// If k-1 is S-type (text[k-1] < text[k]), queue -k to save for the caller.
		// If k is zero, k-1 doesn't exist, so we only need to leave it
		// for the caller. The caller can't tell the difference between
		// an empty slot and a non-empty zero, but there's no need
		// to distinguish them anyway: the final suffix array will end up
		// with one zero somewhere, and that will be a real zero.
		k := j - 1
		c1 := text[k]
		if k > 0 {
			if c0 := text[k-1]; c0 < c1 {
				k = -k
			}
		}

		if cB != c1 {
			bucket[cB] = b
			cB = c1
			b = bucket[cB]
		}
		sa[b] = int32(k)
		b++
	}
}

func induceS_32(text []int32, sa, freq, bucket []int32) {
	// Initialize positions for right side of character buckets.
	bucketMax_32(text, freq, bucket)

	cB := int32(0)
	b := bucket[cB]

	for i := len(sa) - 1; i >= 0; i-- {
		j := int(sa[i])
		if j >= 0 {
			// Skip non-flagged entry.
			// (This loop can't see an empty entry; 0 means the real zero index.)
			continue
		}

		// Negative j is a work queue entry; rewrite to positive j for final suffix array.
		j = -j
		sa[i] = int32(j)

		// Index j was on work queue (encoded as -j but now decoded),
		// meaning k := j-1 is L-type,
		// so we can now place k correctly into sa.
		// If k-1 is S-type, queue -k for processing later in this loop.
		// If k-1 is L-type (text[k-1] > text[k]), queue k to save for the caller.
		// If k is zero, k-1 doesn't exist, so we only need to leave it
		// for the caller.
		k := j - 1
		c1 := text[k]
		if k > 0 {
			if c0 := text[k-1]; c0 <= c1 {
				k = -k
			}
		}

		if cB != c1 {
			bucket[cB] = b
			cB = c1
			b = bucket[cB]
		}
		b--
		sa[b] = int32(k)
	}
}
