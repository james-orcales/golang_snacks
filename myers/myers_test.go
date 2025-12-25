package myers

import (
	"fmt"
	"slices"
	"testing"

	"github.com/james-orcales/golang_snacks/invariant"
	"github.com/james-orcales/golang_snacks/myers/snap"
)

func TestMain(m *testing.M) {
	invariant.RunTestMain(m)
}

func TestLineDiff(t *testing.T) {
	check := func(t *testing.T, old, new string, snapshot snap.Snapshot) {
		t.Helper()
		d := New(old, new)
		actual := fmt.Sprint("\n", d.LineDiff())
		if !snapshot.IsEqual(actual) {
			t.Error("Snapshot mismatch")
		}
	}

	entries := []struct {
		name, old, new string
		snapshot       snap.Snapshot
	}{
		{"EmptyToEmpty", "", "", snap.Init(`
`)},
		{"EmptyToDeletion", "x", "", snap.Init(`
-x`)},
		{"EmptyToInsertion", "", "x", snap.Init(`
+x`)},
		{"IdentitySingleCharacter", "x", "x", snap.Init(`
 x`)},
		{"DeleteDoubleInsertSingle", "xx", "x", snap.Init(`
-xx
+x`)},
		{"DeleteSingleInsertDouble", "x", "xx", snap.Init(`
-x
+xx`)},
		{"ReplaceXYWithXX", "xy", "xx", snap.Init(`
-xy
+xx`)},
		{"ReplaceXXWithXY", "xx", "xy", snap.Init(`
-xx
+xy`)},
		{"ReplaceYXWithXX", "yx", "xx", snap.Init(`
-yx
+xx`)},
		{"ReplaceXXWithYX", "xx", "yx", snap.Init(`
-xx
+yx`)},
		{"ReplaceXYWithXZ", "xy", "xz", snap.Init(`
-xy
+xz`)},

		// === MULTILINE: BLOCK INSERTION/DELETION ==========================================================

		{
			"InsertMiddleLine",
			`Line 1

Line 2
Line 3`,

			`Line 1

Line 1.5
Line 2
Line 3`,

			snap.Init(`
 Line 1
 
+Line 1.5
 Line 2
 Line 3`),
		},
		{
			"DeleteMiddleLine",
			`Header
Body
Footer`,

			`Header

Footer`,

			snap.Init(`
 Header
-Body
+
 Footer`),
		},
		{
			"ReplaceMiddleLine",
			`Alpha
Beta
Gamma`,

			`Alpha
Delta
Gamma`,

			snap.Init(`
 Alpha
-Beta
+Delta
 Gamma`),
		},

		// === CODE: SYNTAX & LOGIC =========================================================================

		{
			"ReplaceIntegerLiteral",
			`x := 10`,
			`x := 20`,
			snap.Init(`
-x := 10
+x := 20`),
		},
		{
			"InvertConditionAndReturn",
			`if check { return true }`,
			`if !check { return false }`,
			snap.Init(`
-if check { return true }
+if !check { return false }`),
		},
		{
			"RenameFunction",
			`func old_name() {}`,
			`func new_name() {}`,
			snap.Init(`
-func old_name() {}
+func new_name() {}`),
		},
		{
			"UpdateCommentStatus",
			`// TODO: Fix this`,
			`// FIXED: Fixed this`,
			snap.Init(`
-// TODO: Fix this
+// FIXED: Fixed this`),
		},
		{
			"ChangeImportedModule",
			`import "core:fmt"`,
			`import "core:log"`,
			snap.Init(`
-import "core:fmt"
+import "core:log"`),
		},

		// === STRUCTURED DATA (JSON/YAML) ==================================================================

		{
			"ToggleBooleanField",
			`{"id": 1, "active": true}`,
			`{"id": 1, "active": false}`,
			snap.Init(`
-{"id": 1, "active": true}
+{"id": 1, "active": false}`),
		},
		{
			"AppendArrayElement",
			`{"items": ["a", "b"]}`,
			`{"items": ["a", "b", "c"]}`,
			snap.Init(`
-{"items": ["a", "b"]}
+{"items": ["a", "b", "c"]}`),
		},
		{
			"RenameEnvironment",
			`name: test

version: 1.0`,

			`name: production

version: 1.0`,

			snap.Init(`
-name: test
+name: production
 
 version: 1.0`),
		},
		{
			"ModifyHtmlListItemAttribute",
			`<ul>
	<li>Item A</li>
	<li>Item B</li>

</ul>`,
			`<ul>
	<li>Item A</li>
	<li class="active">Item B</li>
</ul>`,
			snap.Init(`
 <ul>
 	<li>Item A</li>
-	<li>Item B</li>
-
+	<li class="active">Item B</li>
 </ul>`),
		},
		{
			"ModifySingleLineInBlock",
			`Line A

Line B
Line C
Line D
Line E`,

			`Line A

Line B
Line C Modified
Line D
Line E`,

			snap.Init(`
 Line A
 
 Line B
-Line C
+Line C Modified
 Line D
 Line E`),
		},
		{
			"ExpandFunctionBody",
			`function add(a, b) {
    return a + b
}`,

			`function add(a, b) {
    let sum = a + b
    return sum
}`,

			snap.Init(`
 function add(a, b) {
-    return a + b
+    let sum = a + b
+    return sum
 }`),
		},
		{
			"UpdateHtmlTitleText",
			`<div>
	<h1>Title</h1>
	<p>Paragraph</p>
	<testEditsAreReversibleter>Footer</testEditsAreReversibleter>
</div>`,

			`<div>
	<h1>Title Updated</h1>
	<p>Paragraph</p>
	<testEditsAreReversibleter>Footer</testEditsAreReversibleter>
</div>`,

			snap.Init(`
 <div>
-	<h1>Title</h1>
+	<h1>Title Updated</h1>
 	<p>Paragraph</p>
 	<testEditsAreReversibleter>Footer</testEditsAreReversibleter>
 </div>`),
		},
		{
			"ComplexYamlUserUpdates",
			`users:

- name: Alice
  age: 30
  role: admin

- name: Bob
  age: 25
  role: user

- name: Princess
  age: 1
  role: pet`,

			`users:

- name: Alice
  age: 31
  role: admin

- name: Bob
  age: 25
  role: moderator,

- name: Melody
  age: 2
  role: pet`,

			snap.Init(`
 users:
 
 - name: Alice
-  age: 30
+  age: 31
   role: admin
 
 - name: Bob
   age: 25
-  role: user
+  role: moderator,
 
-- name: Princess
-  age: 1
+- name: Melody
+  age: 2
   role: pet`),
		},
		{
			"ModifySqlOrderAndLimit",
			`SELECT id, name, email

FROM users
WHERE active = 1
ORDER BY name
LIMIT 10;`,

			`SELECT id, name, email

FROM users
WHERE active = 1
ORDER BY created_at DESC
LIMIT 20;`,

			snap.Init(`
 SELECT id, name, email
 
 FROM users
 WHERE active = 1
-ORDER BY name
-LIMIT 10;
+ORDER BY created_at DESC
+LIMIT 20;`),
		},
		{
			"UpdateTryExceptLogic",
			`try:
	do_something()

except Exception as e:
	log(e)

finally:
	cleanup()`,

			`try:
	do_something_critical()

except Exception as e:
	log_error(e)

finally:
	leak()`,

			snap.Init(`
 try:
-	do_something()
+	do_something_critical()
 
 except Exception as e:
-	log(e)
+	log_error(e)
 
 finally:
-	cleanup()
+	leak()`),
		},
		{
			"LicenseChange",
			`MIT License

Copyright (c) 2025 Danzig James Orcales

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`,
			`zlib License

Copyright (c) 2025 Danzig James Orcales. All rights reserved.

This software is provided 'as-is', without any express or implied
warranty. In no event will the authors be held liable for any damages
arising from the use of this software.

Permission is granted to anyone to use this software for any purpose,
including commercial applications, and to alter it and redistribute it
freely, subject to the following restrictions:

1. The origin of this software must not be misrepresented; you must not
   claim that you wrote the original software. If you use this software
   in a product, an acknowledgment in the product documentation would be
   appreciated but is not required.
2. Altered source versions must be plainly marked as such, and must not be
   misrepresented as being the original software.
3. This notice may not be removed or altered from any source distribution.`,
			// This is the exact same diff as git!
			snap.Init(`
-MIT License
+zlib License
 
-Copyright (c) 2025 Danzig James Orcales
+Copyright (c) 2025 Danzig James Orcales. All rights reserved.
 
-Permission is hereby granted, free of charge, to any person obtaining a copy
-of this software and associated documentation files (the "Software"), to deal
-in the Software without restriction, including without limitation the rights
-to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
-copies of the Software, and to permit persons to whom the Software is
-furnished to do so, subject to the following conditions:
+This software is provided 'as-is', without any express or implied
+warranty. In no event will the authors be held liable for any damages
+arising from the use of this software.
 
-The above copyright notice and this permission notice shall be included in all
-copies or substantial portions of the Software.
+Permission is granted to anyone to use this software for any purpose,
+including commercial applications, and to alter it and redistribute it
+freely, subject to the following restrictions:
 
-THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
-IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
-FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
-AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
-LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
-OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
-SOFTWARE.
+1. The origin of this software must not be misrepresented; you must not
+   claim that you wrote the original software. If you use this software
+   in a product, an acknowledgment in the product documentation would be
+   appreciated but is not required.
+2. Altered source versions must be plainly marked as such, and must not be
+   misrepresented as being the original software.
+3. This notice may not be removed or altered from any source distribution.`),
		},
	}
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			check(t, entry.old, entry.new, entry.snapshot)
		})
	}
}

func TestDiff(t *testing.T) {
	check := func(t *testing.T, old, new string, snapshot snap.Snapshot) {
		t.Helper()
		d := New(old, new)
		if !snapshot.IsEqual(d.Diff()) {
			t.Error("Snapshot mismatch")
		}
	}
	entries := []struct {
		name, old, new string
		snapshot       snap.Snapshot
	}{
		{"EmptyToEmpty", "", "", snap.Init(``)},
		{"EmptyToX", "", "x", snap.Init(`+"x"`)},
		{"XToEmpty", "x", "", snap.Init(`-"x"`)},
		{"XToX", "x", "x", snap.Init(` "x"`)},
		{"XToDoubleX", "x", "xx", snap.Init(` "x"+"x"`)},
		{"DoubleXToX", "xx", "x", snap.Init(` "x"-"x"`)},
		{"DoubleXToXY", "xx", "xy", snap.Init(` "x"-"x"+"y"`)},
		{"XYToDoubleX", "xy", "xx", snap.Init(` "x"-"y"+"x"`)},
		{"DoubleXToYX", "xx", "yx", snap.Init(`-"x"+"y" "x"`)},
		{"YXToDoubleX", "yx", "xx", snap.Init(`-"y"+"x" "x"`)},
		{"XYToXZ", "xy", "xz", snap.Init(` "x"-"y"+"z"`)},

		// === BLOG POST EXAMPLES ======================================================================================================================

		{"DogToCatInHat", "The dog in the hat.", "The cat in the hat.", snap.Init(` "The "-"dog"+"cat" " in the hat."`)},
		{"AddFurryToCat", "The cat in the hat.", "The furry cat in the hat.", snap.Init(` "The "+"furry " "cat in the hat."`)},
		{"TruncateCatSentence", "The cat in the hat.", "The cat.", snap.Init(` "The cat"-" in the hat" "."`)},
		{"HappyCatInBlackHat", "The cat in the hat.", "The happy cat in the black hat.", snap.Init(` "The "+"happy " "cat in the"+" black" " hat."`)},
		{"OxToCatInHat", "The ox in the box.", "The cat in the hat.", snap.Init(` "The "-"ox"+"cat" " in the "-"box"+"hat" "."`)},

		// === CUSTOM EXAMPLES =========================================================================================================================

		{
			"ChaseSceneReplacement",
			"A dog chased a rat across the yard.",
			"The cat chased the mouse through the garden.",
			snap.Init(`-"A"+"The" " "-"dog"+"cat" " chased "-"a ra" "t"+"he" " "-"acr"+"m" "o"+"u" "s"-"s"+"e through" " the "-"y"+"g" "ard"+"en" "."`),
		},
		{
			"NumberWordSubstitution",
			"1 two 3 four five 6 seven",
			"one two three four five six seven",
			snap.Init(`-"1"+"one" " two "-"3"+"three" " four five "-"6"+"six" " seven"`),
		},
		{
			"ComplexMixedReplacement",
			"AAAAAA111BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBzyyyjCCCCCCCC333DDDDDD",
			"AAAAAAxxxBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBxyyyiCCCCCCCCzzzDDDDDD",
			snap.Init(` "AAAAAA"-"111"+"xxx" "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"-"z"+"x" "yyy"-"j"+"i" "CCCCCCCC"-"333"+"zzz" "DDDDDD"`),
		},
		{
			"LoremIpsumExpansion",
			"Lorem ipsum dolor sit amet, with some extra words added here, consectetur elit.",
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
			snap.Init(` "Lorem ipsum dolor sit amet, "-"with some extra words added here, " "consectetur"+" adipiscing" " elit."`),
		},
	}
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			check(t, entry.old, entry.new, entry.snapshot)
		})
	}
}

func TestAlgorithmDiff(t *testing.T) {
	check := func(t *testing.T, old, new string, snapshot snap.Snapshot) {
		t.Helper()
		d := New(old, new)
		d.AlgorithmDiff()
		if !snapshot.IsEqual(d.String()) {
			t.Error("Snapshot mismatch")
		}
	}
	entries := []struct {
		name, old, new string
		snapshot       snap.Snapshot
	}{
		{"EmptyToEmpty", "", "", snap.Init(``)},
		{"EmptyToX", "", "x", snap.Init(`+"x"`)},
		{"XToEmpty", "x", "", snap.Init(`-"x"`)},
		{"XToX", "x", "x", snap.Init(` "x"`)},
		{"XToDoubleX", "x", "xx", snap.Init(` "x"+"x"`)},
		{"DoubleXToX", "xx", "x", snap.Init(` "x"-"x"`)},
		{"DoubleXToXY", "xx", "xy", snap.Init(` "x"-"x"+"y"`)},
		{"XYToDoubleX", "xy", "xx", snap.Init(` "x"-"y"+"x"`)},
		{"DoubleXToYX", "xx", "yx", snap.Init(`+"y" "x"-"x"`)},
		{"YXToDoubleX", "yx", "xx", snap.Init(`-"y" "x"+"x"`)},
		{"XYToXZ", "xy", "xz", snap.Init(` "x"-"y"+"z"`)},

		// === BLOG POST EXAMPLES ==============================================================================================================================

		{"DogToCatInHat", "The dog in the hat.", "The cat in the hat.", snap.Init(` "The "-"d"-"o"-"g"+"c"+"a"+"t" " in the hat."`)},
		{"AddFurryToCat", "The cat in the hat.", "The furry cat in the hat.", snap.Init(` "The "+"f"+"u"+"r"+"r"+"y"+" " "cat in the hat."`)},
		{"TruncateCatSentence", "The cat in the hat.", "The cat.", snap.Init(` "The cat"-" "-"i"-"n"-" "-"t"-"h"-"e"-" "-"h"-"a"-"t" "."`)},
		{"HappyCatInBlackHat", "The cat in the hat.", "The happy cat in the black hat.", snap.Init(` "The "+"h"+"a"+"p"+"p"+"y"+" " "cat in the "+"b"+"l"+"a"+"c"+"k"+" " "hat."`)},
		{"OxToCatInHat", "The ox in the box.", "The cat in the hat.", snap.Init(` "The "-"o"-"x"+"c"+"a"+"t" " in the "-"b"-"o"-"x"+"h"+"a"+"t" "."`)},

		// === CUSTOM EXAMPLES =================================================================================================================================

		{"XYToXZ", "meee", "eeek", snap.Init(`-"m" "eee"+"k"`)},
		{"XYToXZ", "xyzz", "ikzz", snap.Init(`-"x"-"y"+"i"+"k" "zz"`)},
		{"ChaseSceneReplacement", "A dog chased a rat across the yard.", "The cat chased the mouse through the garden.", snap.Init(`-"A"+"T"+"h"+"e" " "-"d"-"o"-"g"+"c"+"a"+"t" " chased "-"a"-" "-"r"-"a" "t"+"h"+"e" " "-"a"-"c"-"r"+"m" "o"+"u" "s"-"s"+"e" " th"+"r"+"o"+"u"+"g"+"h"+" "+"t"+"h" "e "-"y"+"g" "ard"+"e"+"n" "."`)},
		{"NumberWordSubstitution", "1 two 3 four five 6 seven", "one two three four five six seven", snap.Init(`-"1"+"o"+"n"+"e" " two "-"3"+"t"+"h"+"r"+"e"+"e" " four five "-"6"+"s"+"i"+"x" " seven"`)},
		{
			"ComplexMixedReplacement",
			"AAAAAA111BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBzyyyjCCCCCCCC333DDDDDD",
			"AAAAAAxxxBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBxyyyiCCCCCCCCzzzDDDDDD",
			snap.Init(` "AAAAAA"-"1"-"1"-"1"+"x"+"x"+"x" "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"-"z"+"x" "yyy"-"j"+"i" "CCCCCCCC"-"3"-"3"-"3"+"z"+"z"+"z" "DDDDDD"`),
		},
		{
			"LoremIpsumExpansion",
			"Lorem ipsum dolor sit amet, with some extra words added here, consectetur elit.",
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
			snap.Init(` "Lorem ipsum dolor sit amet, "-"w"-"i"-"t"-"h"-" "-"s"-"o"-"m"-"e"-" "-"e"-"x"-"t"-"r"-"a"-" "-"w"-"o"-"r"-"d"-"s"-" "-"a"-"d"-"d"-"e"-"d"-" "-"h"-"e"-"r"-"e"-","-" " "consectetur "+"a"+"d"+"i"+"p"+"i"+"s"+"c"+"i"+"n"+"g"+" " "elit."`),
		},
	}
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			check(t, entry.old, entry.new, entry.snapshot)
		})
	}
}

func TestFindCommonSubstring(t *testing.T) {
	check := func(t *testing.T, a, b string, snapshot snap.Snapshot) {
		t.Helper()
		actual := findCommonSubstring([]rune(a), []rune(b))
		if !snapshot.IsEqual(string(actual)) {
			t.Error("Snapshot Mismatch")
		}
	}
	type Entry struct {
		name, a, b string
		snapshot   snap.Snapshot
	}
	entries := []Entry{
		{"BothStringsEmpty", "", "", snap.Init(``)},
		{"FirstNonEmptySecondEmpty", "a", "", snap.Init(``)},
		{"FirstEmptySecondNonEmpty", "", "a", snap.Init(``)},
		{"SingleCharacterMatch", "a", "a", snap.Init(`a`)},

		{"IdenticalStrings", "same", "same", snap.Init(`same`)},
		{"ReversedStringsNoMatch", "same", "emas", snap.Init(``)},
		{"ShorterPrefixOfLonger", "long", "longer", snap.Init(`long`)},
		{"LongerContainsShorterPrefix", "longer", "long", snap.Init(`long`)},
		{
			"CommonSubstringBelowHalfLengthThreshold",
			"substr_is_too_smallzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			"substr_is_too_smalljjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj",
			snap.Init(``),
		},

		{"UnicodeAccentedOverlapLeft", "mañana", "ranaña", snap.Init(`aña`)},
		{"UnicodeAccentedOverlapRight", "ranaña", "mañana", snap.Init(`ana`)},

		{"EvenLengthMiddleMatch", "xeven", "zeven", snap.Init(`even`)},
		{"EvenLengthWithPaddingTwo", "xxeven", "zzeven", snap.Init(`even`)},
		{"EvenLengthWithPaddingThree", "xxxeven", "zzzeven", snap.Init(`even`)},
		{"OddLengthMiddleMatch", "xodd", "zodd", snap.Init(`odd`)},
		{"OddLengthWithPaddingTwo", "xxodd", "zzodd", snap.Init(`odd`)},
		{"OddLengthWithPaddingThree", "xxxodd", "zzzodd", snap.Init(`odd`)},

		{"EvenLengthSuffixMatch", "evenx", "eveny", snap.Init(`even`)},
		{"EvenLengthSuffixWithPaddingTwo", "evenxx", "evenyy", snap.Init(`even`)},
		{"EvenLengthSuffixWithPaddingThree", "evenxxx", "evenyyy", snap.Init(`even`)},
		{"OddLengthSuffixMatch", "oddx", "oddy", snap.Init(`odd`)},
		{"OddLengthSuffixWithPaddingTwo", "oddxx", "oddyy", snap.Init(`odd`)},
		{"OddLengthSuffixWithPaddingThree", "oddxxx", "oddyyy", snap.Init(`odd`)},

		{"SentenceContainedWithPrefix", "cat in the hat.", "furry cat in the hat.", snap.Init(`cat in the hat.`)},

		{"UnicodeSingleRuneMatch", "á", "á", snap.Init(`á`)},
		{"UnicodeAccentedVsAsciiNoMatch", "á", "a", snap.Init(``)},
		{"CjkIdenticalStrings", "你好", "你好", snap.Init(`你好`)},
		{"CjkCommonSuffix", "再见你好", "朋友你好", snap.Init(`你好`)},
		{"EmojiSingleMatch", "😀", "😀", snap.Init(`😀`)},
		{"EmojiCommonSuffix", "😀✅", "😄✅", snap.Init(`✅`)},
		{"UnicodeMathematicalScriptIdentical", "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", snap.Init(`𝔘𝔫𝔦𝔠𝔬𝔡𝔢`)},
	}

	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			check(t, entry.a, entry.b, entry.snapshot)
		})
	}
}

func TestFindCommonPrefix(t *testing.T) {
	check := func(t *testing.T, a, b []rune, snapshot snap.Snapshot) {
		t.Helper()
		actual := findCommonPrefix(a, b)
		mirror := findCommonPrefix(b, a)
		if !snapshot.IsEqual(string(actual)) {
			t.Error("Snapshot Mismatch")
		}
		if !slices.Equal(actual, mirror) {
			t.Error("Common prefix changes with flipped arguments")
		}
		if len(actual) > 0 && (!runesHavePrefix(a, actual) || !runesHavePrefix(b, actual)) {
			t.Error("Common prefix was not found")
		}
	}
	type Entry struct {
		name, a, b string
		snapshot   snap.Snapshot
	}

	entries := []Entry{
		{"BothStringsEmpty", "", "", snap.Init(``)},
		{"FirstNonEmptySecondEmpty", "a", "", snap.Init(``)},
		{"FirstEmptySecondNonEmpty", "", "a", snap.Init(``)},
		{"SingleCharacterMatch", "a", "a", snap.Init(``)},

		{"IdenticalStrings", "same", "same", snap.Init(``)},
		{"ReversedStringsNoMatch", "same", "emas", snap.Init(``)},
		{"ShorterPrefixOfLonger", "long", "longer", snap.Init(``)},
		{"LongerContainsShorterPrefix", "longer", "long", snap.Init(``)},

		{"UnicodeAccentedOverlapLeft", "mañana", "ranaña", snap.Init(``)},
		{"UnicodeAccentedOverlapRight", "ranaña", "mañana", snap.Init(``)},

		{"EvenLengthMiddleMatch", "xeven", "zeven", snap.Init(``)},
		{"EvenLengthWithPaddingTwo", "xxeven", "zzeven", snap.Init(``)},
		{"EvenLengthWithPaddingThree", "xxxeven", "zzzeven", snap.Init(``)},
		{"OddLengthMiddleMatch", "xodd", "zodd", snap.Init(``)},
		{"OddLengthWithPaddingTwo", "xxodd", "zzodd", snap.Init(``)},
		{"OddLengthWithPaddingThree", "xxxodd", "zzzodd", snap.Init(``)},

		{"EvenLengthSuffixMatch", "evenx", "eveny", snap.Init(`even`)},
		{"EvenLengthSuffixWithPaddingTwo", "evenxx", "evenyy", snap.Init(`even`)},
		{"EvenLengthSuffixWithPaddingThree", "evenxxx", "evenyyy", snap.Init(`even`)},
		{"OddLengthSuffixMatch", "oddx", "oddy", snap.Init(`odd`)},
		{"OddLengthSuffixWithPaddingTwo", "oddxx", "oddyy", snap.Init(`odd`)},
		{"OddLengthSuffixWithPaddingThree", "oddxxx", "oddyyy", snap.Init(`odd`)},

		{"SentenceContainedWithPrefix", "cat in the hat.", "furry cat in the hat.", snap.Init(``)},

		{"UnicodeSingleRuneMatch", "á", "á", snap.Init(``)},
		{"UnicodeAccentedVsAsciiNoMatch", "á", "a", snap.Init(``)},
		{"CjkIdenticalStrings", "你好", "你好", snap.Init(``)},
		{"CjkCommonSuffix", "再见你好", "朋友你好", snap.Init(``)},
		{"EmojiSingleMatch", "😀", "😀", snap.Init(``)},
		{"EmojiCommonSuffix", "😀✅", "😄✅", snap.Init(``)},
		{"UnicodeMathematicalScriptIdentical", "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", snap.Init(``)},
	}

	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			check(t, []rune(entry.a), []rune(entry.b), entry.snapshot)
		})
	}
}

func TestFindCommonSuffix(t *testing.T) {
	check := func(t *testing.T, a, b []rune, snapshot snap.Snapshot) {
		t.Helper()
		actual := findCommonSuffix(a, b)
		mirror := findCommonSuffix(b, a)
		if !snapshot.IsEqual(string(actual)) {
			t.Error("Snapshot Mismatch")
		}
		if !slices.Equal(actual, mirror) {
			t.Error("Common suffix changes with flipped arguments")
		}
		if len(actual) > 0 && (!runesHaveSuffix(a, actual) || !runesHaveSuffix(b, actual)) {
			t.Error("Common suffix was not found")
		}
	}
	type Entry struct {
		name, a, b string
		snapshot   snap.Snapshot
	}

	entries := []Entry{
		{"EmptyBoth", "", "", snap.Init(``)},
		{"OnlyName", "a", "", snap.Init(``)},
		{"OnlyOriginalText", "", "a", snap.Init(``)},
		{"SameSingleLetter", "a", "a", snap.Init(``)},

		{"ExactMatch", "same", "same", snap.Init(``)},
		{"ReversedLetters", "same", "emas", snap.Init(``)},
		{"ShortVsLong", "long", "longer", snap.Init(``)},
		{"LongVsShort", "longer", "long", snap.Init(``)},

		{"PrefixEven", "xeven", "zeven", snap.Init(`even`)},
		{"DoublePrefixEven", "xxeven", "zzeven", snap.Init(`even`)},
		{"TriplePrefixEven", "xxxeven", "zzzeven", snap.Init(`even`)},
		{"PrefixOdd", "xodd", "zodd", snap.Init(`odd`)},
		{"DoublePrefixOdd", "xxodd", "zzodd", snap.Init(`odd`)},
		{"TriplePrefixOdd", "xxxodd", "zzzodd", snap.Init(`odd`)},

		{"EvenSuffixDiff", "evenx", "eveny", snap.Init(``)},
		{"DoubleEvenSuffixDiff", "evenxx", "evenyy", snap.Init(``)},
		{"TripleEvenSuffixDiff", "evenxxx", "evenyyy", snap.Init(``)},
		{"OddSuffixDiff", "oddx", "oddy", snap.Init(``)},
		{"DoubleOddSuffixDiff", "oddxx", "oddyy", snap.Init(``)},
		{"TripleOddSuffixDiff", "oddxxx", "oddyyy", snap.Init(``)},

		{"CatPhrase", "cat in the hat.", "furry cat in the hat.", snap.Init(``)},

		{"AccentedMatch", "á", "á", snap.Init(``)},
		{"AccentedToPlain", "á", "a", snap.Init(``)},
		{"ChineseMatch", "你好", "你好", snap.Init(``)},
		{"ChinesePartialMatch", "再见你好", "朋友你好", snap.Init(`你好`)},
		{"EmojiMatch", "😀", "😀", snap.Init(``)},
		{"EmojiPartialMatch", "😀✅", "😄✅", snap.Init(`✅`)},
		{"SpanishPartialMatch", "mañana", "ranaña", snap.Init(`a`)},
		{"UnicodeMatch", "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", snap.Init(``)},
	}

	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			check(t, []rune(entry.a), []rune(entry.b), entry.snapshot)
		})
	}
}
