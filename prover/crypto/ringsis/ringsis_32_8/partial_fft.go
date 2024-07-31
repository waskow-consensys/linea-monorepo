// Code generated by bavard DO NOT EDIT

package ringsis_32_8

import (
	"github.com/consensys/zkevm-monorepo/prover/maths/field"
)

var partialFFT = []func(a, twiddles []field.Element){
	partialFFT_0,
	partialFFT_1,
}

func partialFFT_0(a, twiddles []field.Element) {
}

func partialFFT_1(a, twiddles []field.Element) {
	a[16].Mul(&a[16], &twiddles[0])
	a[17].Mul(&a[17], &twiddles[0])
	a[18].Mul(&a[18], &twiddles[0])
	a[19].Mul(&a[19], &twiddles[0])
	a[20].Mul(&a[20], &twiddles[0])
	a[21].Mul(&a[21], &twiddles[0])
	a[22].Mul(&a[22], &twiddles[0])
	a[23].Mul(&a[23], &twiddles[0])
	a[24].Mul(&a[24], &twiddles[0])
	a[25].Mul(&a[25], &twiddles[0])
	a[26].Mul(&a[26], &twiddles[0])
	a[27].Mul(&a[27], &twiddles[0])
	a[28].Mul(&a[28], &twiddles[0])
	a[29].Mul(&a[29], &twiddles[0])
	a[30].Mul(&a[30], &twiddles[0])
	a[31].Mul(&a[31], &twiddles[0])
	field.Butterfly(&a[0], &a[16])
	field.Butterfly(&a[1], &a[17])
	field.Butterfly(&a[2], &a[18])
	field.Butterfly(&a[3], &a[19])
	field.Butterfly(&a[4], &a[20])
	field.Butterfly(&a[5], &a[21])
	field.Butterfly(&a[6], &a[22])
	field.Butterfly(&a[7], &a[23])
	field.Butterfly(&a[8], &a[24])
	field.Butterfly(&a[9], &a[25])
	field.Butterfly(&a[10], &a[26])
	field.Butterfly(&a[11], &a[27])
	field.Butterfly(&a[12], &a[28])
	field.Butterfly(&a[13], &a[29])
	field.Butterfly(&a[14], &a[30])
	field.Butterfly(&a[15], &a[31])
	a[8].Mul(&a[8], &twiddles[1])
	a[9].Mul(&a[9], &twiddles[1])
	a[10].Mul(&a[10], &twiddles[1])
	a[11].Mul(&a[11], &twiddles[1])
	a[12].Mul(&a[12], &twiddles[1])
	a[13].Mul(&a[13], &twiddles[1])
	a[14].Mul(&a[14], &twiddles[1])
	a[15].Mul(&a[15], &twiddles[1])
	a[24].Mul(&a[24], &twiddles[2])
	a[25].Mul(&a[25], &twiddles[2])
	a[26].Mul(&a[26], &twiddles[2])
	a[27].Mul(&a[27], &twiddles[2])
	a[28].Mul(&a[28], &twiddles[2])
	a[29].Mul(&a[29], &twiddles[2])
	a[30].Mul(&a[30], &twiddles[2])
	a[31].Mul(&a[31], &twiddles[2])
	field.Butterfly(&a[0], &a[8])
	field.Butterfly(&a[1], &a[9])
	field.Butterfly(&a[2], &a[10])
	field.Butterfly(&a[3], &a[11])
	field.Butterfly(&a[4], &a[12])
	field.Butterfly(&a[5], &a[13])
	field.Butterfly(&a[6], &a[14])
	field.Butterfly(&a[7], &a[15])
	field.Butterfly(&a[16], &a[24])
	field.Butterfly(&a[17], &a[25])
	field.Butterfly(&a[18], &a[26])
	field.Butterfly(&a[19], &a[27])
	field.Butterfly(&a[20], &a[28])
	field.Butterfly(&a[21], &a[29])
	field.Butterfly(&a[22], &a[30])
	field.Butterfly(&a[23], &a[31])
	a[4].Mul(&a[4], &twiddles[3])
	a[5].Mul(&a[5], &twiddles[3])
	a[6].Mul(&a[6], &twiddles[3])
	a[7].Mul(&a[7], &twiddles[3])
	a[12].Mul(&a[12], &twiddles[4])
	a[13].Mul(&a[13], &twiddles[4])
	a[14].Mul(&a[14], &twiddles[4])
	a[15].Mul(&a[15], &twiddles[4])
	a[20].Mul(&a[20], &twiddles[5])
	a[21].Mul(&a[21], &twiddles[5])
	a[22].Mul(&a[22], &twiddles[5])
	a[23].Mul(&a[23], &twiddles[5])
	a[28].Mul(&a[28], &twiddles[6])
	a[29].Mul(&a[29], &twiddles[6])
	a[30].Mul(&a[30], &twiddles[6])
	a[31].Mul(&a[31], &twiddles[6])
	field.Butterfly(&a[0], &a[4])
	field.Butterfly(&a[1], &a[5])
	field.Butterfly(&a[2], &a[6])
	field.Butterfly(&a[3], &a[7])
	field.Butterfly(&a[8], &a[12])
	field.Butterfly(&a[9], &a[13])
	field.Butterfly(&a[10], &a[14])
	field.Butterfly(&a[11], &a[15])
	field.Butterfly(&a[16], &a[20])
	field.Butterfly(&a[17], &a[21])
	field.Butterfly(&a[18], &a[22])
	field.Butterfly(&a[19], &a[23])
	field.Butterfly(&a[24], &a[28])
	field.Butterfly(&a[25], &a[29])
	field.Butterfly(&a[26], &a[30])
	field.Butterfly(&a[27], &a[31])
	a[2].Mul(&a[2], &twiddles[7])
	a[3].Mul(&a[3], &twiddles[7])
	a[6].Mul(&a[6], &twiddles[8])
	a[7].Mul(&a[7], &twiddles[8])
	a[10].Mul(&a[10], &twiddles[9])
	a[11].Mul(&a[11], &twiddles[9])
	a[14].Mul(&a[14], &twiddles[10])
	a[15].Mul(&a[15], &twiddles[10])
	a[18].Mul(&a[18], &twiddles[11])
	a[19].Mul(&a[19], &twiddles[11])
	a[22].Mul(&a[22], &twiddles[12])
	a[23].Mul(&a[23], &twiddles[12])
	a[26].Mul(&a[26], &twiddles[13])
	a[27].Mul(&a[27], &twiddles[13])
	a[30].Mul(&a[30], &twiddles[14])
	a[31].Mul(&a[31], &twiddles[14])
	field.Butterfly(&a[0], &a[2])
	field.Butterfly(&a[1], &a[3])
	field.Butterfly(&a[4], &a[6])
	field.Butterfly(&a[5], &a[7])
	field.Butterfly(&a[8], &a[10])
	field.Butterfly(&a[9], &a[11])
	field.Butterfly(&a[12], &a[14])
	field.Butterfly(&a[13], &a[15])
	field.Butterfly(&a[16], &a[18])
	field.Butterfly(&a[17], &a[19])
	field.Butterfly(&a[20], &a[22])
	field.Butterfly(&a[21], &a[23])
	field.Butterfly(&a[24], &a[26])
	field.Butterfly(&a[25], &a[27])
	field.Butterfly(&a[28], &a[30])
	field.Butterfly(&a[29], &a[31])
	a[1].Mul(&a[1], &twiddles[15])
	a[3].Mul(&a[3], &twiddles[16])
	a[5].Mul(&a[5], &twiddles[17])
	a[7].Mul(&a[7], &twiddles[18])
	a[9].Mul(&a[9], &twiddles[19])
	a[11].Mul(&a[11], &twiddles[20])
	a[13].Mul(&a[13], &twiddles[21])
	a[15].Mul(&a[15], &twiddles[22])
	a[17].Mul(&a[17], &twiddles[23])
	a[19].Mul(&a[19], &twiddles[24])
	a[21].Mul(&a[21], &twiddles[25])
	a[23].Mul(&a[23], &twiddles[26])
	a[25].Mul(&a[25], &twiddles[27])
	a[27].Mul(&a[27], &twiddles[28])
	a[29].Mul(&a[29], &twiddles[29])
	a[31].Mul(&a[31], &twiddles[30])
	field.Butterfly(&a[0], &a[1])
	field.Butterfly(&a[2], &a[3])
	field.Butterfly(&a[4], &a[5])
	field.Butterfly(&a[6], &a[7])
	field.Butterfly(&a[8], &a[9])
	field.Butterfly(&a[10], &a[11])
	field.Butterfly(&a[12], &a[13])
	field.Butterfly(&a[14], &a[15])
	field.Butterfly(&a[16], &a[17])
	field.Butterfly(&a[18], &a[19])
	field.Butterfly(&a[20], &a[21])
	field.Butterfly(&a[22], &a[23])
	field.Butterfly(&a[24], &a[25])
	field.Butterfly(&a[26], &a[27])
	field.Butterfly(&a[28], &a[29])
	field.Butterfly(&a[30], &a[31])
}
