// Copyright 2017 Aerospike, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aerospike_test

import (
	"fmt"
	"time"

	. "github.com/aerospike/aerospike-client-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// ALL tests are isolated by SetName and Key, which are 50 random characters
var _ = Describe("predexp operations", func() {
	initTestVars()

	const keyCount = 1000

	var ns = "test"
	var set = randString(10)
	var wpolicy = NewWritePolicy(0, 0)

	starbucks := [][2]float64{
		{-122.1708441, 37.4241193},
		{-122.1492040, 37.4273569},
		{-122.1441078, 37.4268202},
		{-122.1251714, 37.4130590},
		{-122.0964289, 37.4218102},
		{-122.0776641, 37.4158199},
		{-122.0943475, 37.4114654},
		{-122.1122861, 37.4028493},
		{-122.0947230, 37.3909250},
		{-122.0831037, 37.3876090},
		{-122.0707119, 37.3787855},
		{-122.0303178, 37.3882739},
		{-122.0464861, 37.3786236},
		{-122.0582128, 37.3726980},
		{-122.0365083, 37.3676930},
	}

	var gaptime int64

	BeforeEach(func() {
		wpolicy = NewWritePolicy(0, 24*60*60)

		for ii := 0; ii < keyCount; ii++ {

			// On iteration 333 we pause for a few mSec and note the
			// time.  Later we can check last_update time for either
			// side of this gap ...
			//
			// Also, we update the WritePolicy to never expire so
			// records w/ 0 TTL can be counted later.
			//
			if ii == 333 {
				<-time.After(5 * time.Millisecond)
				gaptime = time.Now().UnixNano()
				<-time.After(5 * time.Millisecond)

				wpolicy = NewWritePolicy(0, TTLDontExpire)
			}

			key, err := NewKey(ns, set, ii)
			Expect(err).ToNot(HaveOccurred())

			lng := -122.0 + (0.1 * float64(ii))
			lat := 37.5 + (0.1 * float64(ii))
			pointstr := fmt.Sprintf(
				"{ \"type\": \"Point\", \"coordinates\": [%f, %f] }",
				lng, lat)

			var regionstr string
			if ii < len(starbucks) {
				regionstr = fmt.Sprintf(
					"{ \"type\": \"AeroCircle\", "+
						"  \"coordinates\": [[%f, %f], 3000.0 ] }",
					starbucks[ii][0], starbucks[ii][1])
			} else {
				// Somewhere off Africa ...
				regionstr =
					"{ \"type\": \"AeroCircle\", " +
						"  \"coordinates\": [[0.0, 0.0], 3000.0 ] }"
			}

			listval := []int{}
			for _, ff := range []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31} {
				if ii % ff == 0 {
					listval = append(listval, ff)
				}
			}

			ballast := make([]byte, ii*16)

			bins := BinMap{
				"intval":  ii,
				"strval":  fmt.Sprintf("0x%04x", ii),
				"modval":  ii % 10,
				"locval":  NewGeoJSONValue(pointstr),
				"rgnval":  NewGeoJSONValue(regionstr),
				"lstval":  listval,
				"ballast": ballast,
			}
			err = client.Put(wpolicy, key, bins)
		}

		idxTask, err := client.CreateIndex(
			wpolicy, ns, set, "intval", "intval", NUMERIC)
		Expect(err).ToNot(HaveOccurred())
		Expect(<-idxTask.OnComplete()).ToNot(HaveOccurred())

		idxTask, err = client.CreateIndex(
			wpolicy, ns, set, "strval", "strval", STRING)
		Expect(err).ToNot(HaveOccurred())
		Expect(<-idxTask.OnComplete()).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DropIndex(nil, ns, set, "intval")).ToNot(HaveOccurred())
		Expect(client.DropIndex(nil, ns, set, "strval")).ToNot(HaveOccurred())
	})

	It("server error with top level predexp value node", func() {

		// This statement doesn't form a predicate expression.
		stm := NewStatement(ns, set)
		stm.Addfilter(NewRangeFilter("intval", 0, 400))
		stm.AddPredExp(NewPredExpIntegerValue(8))
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())
		for res := range recordset.Results() {
			Expect(res.Err).To(HaveOccurred())
		}
	})

	It("server error with multiple top-level predexp", func() {

		stm := NewStatement(ns, set)
		stm.Addfilter(NewRangeFilter("intval", 0, 400))
		stm.AddPredExp(NewPredExpIntegerValue(8))
		stm.AddPredExp(NewPredExpIntegerBin("modval"))
		stm.AddPredExp(NewPredExpIntegerGreaterEq())
		stm.AddPredExp(NewPredExpIntegerValue(8))
		stm.AddPredExp(NewPredExpIntegerBin("modval"))
		stm.AddPredExp(NewPredExpIntegerGreaterEq())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())
		for res := range recordset.Results() {
			Expect(res.Err).To(HaveOccurred())
		}
	})

	It("server error with missing child predexp", func() {

		stm := NewStatement(ns, set)
		stm.Addfilter(NewRangeFilter("intval", 0, 400))
		stm.AddPredExp(NewPredExpIntegerValue(8))
		stm.AddPredExp(NewPredExpIntegerGreaterEq()) // needs two children!
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())
		for res := range recordset.Results() {
			Expect(res.Err).To(HaveOccurred())
		}
	})

	It("predexp must additionally filter indexed query results", func() {

		stm := NewStatement(ns, set)
		stm.Addfilter(NewRangeFilter("intval", 0, 400))
		stm.AddPredExp(NewPredExpIntegerValue(8))
		stm.AddPredExp(NewPredExpIntegerBin("modval"))
		stm.AddPredExp(NewPredExpIntegerGreaterEq())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		// The query clause selects [0, 1, ... 400, 401] The predexp
		// only takes mod 8 and 9, should be 2 pre decade or 80 total.

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 80))
	})

	It("predexp must additionally filter indexed query results using Predicate API", func() {

		stm := NewStatement(ns, set)
		stm.Addfilter(NewRangeFilter("intval", 0, 400))
		stm.SetPredicate(BinValue("modval").EqualOrGreaterThan(8))

		// stm.AddPredExp(NewPredExpIntegerValue(8))
		// stm.AddPredExp(NewPredExpIntegerBin("modval"))
		// stm.AddPredExp(NewPredExpIntegerGreaterEq())

		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		// The query clause selects [0, 1, ... 400, 401] The predexp
		// only takes mod 8 and 9, should be 2 pre decade or 80 total.

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 80))
	})

	It("predexp must work with implied scan", func() {

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpStringValue("0x0001"))
		stm.AddPredExp(NewPredExpStringBin("strval"))
		stm.AddPredExp(NewPredExpStringEqual())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 1))
	})

	It("predexp must work with implied scan using Predicate API", func() {

		stm := NewStatement(ns, set)
		stm.SetPredicate(BinValue("strval").Equal("0x0001"))
		// stm.AddPredExp(NewPredExpStringValue("0x0001"))
		// stm.AddPredExp(NewPredExpStringBin("strval"))
		// stm.AddPredExp(NewPredExpStringEqual())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 1))
	})

	It("predexp and or and not must all work", func() {

		stm := NewStatement(ns, set)

		// This returns 999
		stm.AddPredExp(NewPredExpStringValue("0x0001"))
		stm.AddPredExp(NewPredExpStringBin("strval"))
		stm.AddPredExp(NewPredExpStringEqual())
		stm.AddPredExp(NewPredExpNot())

		// This is two per decade
		stm.AddPredExp(NewPredExpIntegerValue(8))
		stm.AddPredExp(NewPredExpIntegerBin("modval"))
		stm.AddPredExp(NewPredExpIntegerGreaterEq())

		// Should be 200
		stm.AddPredExp(NewPredExpAnd(2))

		// Should exactly match 3 values not in prior set
		stm.AddPredExp(NewPredExpStringValue("0x0104"))
		stm.AddPredExp(NewPredExpStringBin("strval"))
		stm.AddPredExp(NewPredExpStringEqual())
		stm.AddPredExp(NewPredExpStringValue("0x0105"))
		stm.AddPredExp(NewPredExpStringBin("strval"))
		stm.AddPredExp(NewPredExpStringEqual())
		stm.AddPredExp(NewPredExpStringValue("0x0106"))
		stm.AddPredExp(NewPredExpStringBin("strval"))
		stm.AddPredExp(NewPredExpStringEqual())

		// 200 + 3
		stm.AddPredExp(NewPredExpOr(4))

		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 203))
	})

	It("predexp regex match must work", func() {

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpStringValue("0x00.[12]"))
		stm.AddPredExp(NewPredExpStringBin("strval"))
		stm.AddPredExp(NewPredExpStringRegex(0))
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		// Should be 32 results:
		// 0x0001, 0x0002,
		// 0x0011, 0x0012,
		// ...
		// 0x00f1, 0x00f2,

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 32))
	})

	It("predexp regex match must work using Predicate API", func() {

		stm := NewStatement(ns, set)
		stm.SetPredicate(BinValue("strval").Regexp("0x00.[12]"))
		// stm.AddPredExp(NewPredExpStringValue("0x00.[12]"))
		// stm.AddPredExp(NewPredExpStringBin("strval"))
		// stm.AddPredExp(NewPredExpStringRegex(0))
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		// Should be 32 results:
		// 0x0001, 0x0002,
		// 0x0011, 0x0012,
		// ...
		// 0x00f1, 0x00f2,

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		Expect(cnt).To(BeNumerically("==", 32))
	})

	It("predexp geo PIR query must work", func() {

		region :=
			"{ " +
				"    \"type\": \"Polygon\", " +
				"    \"coordinates\": [ " +
				"        [[-122.500000, 37.000000],[-121.000000, 37.000000], " +
				"         [-121.000000, 38.080000],[-122.500000, 38.080000], " +
				"         [-122.500000, 37.000000]] " +
				"    ] " +
				"}"

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpGeoJSONValue(region))
		stm.AddPredExp(NewPredExpGeoJSONBin("locval"))
		stm.AddPredExp(NewPredExpGeoJSONWithin())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Correct answer is 6.  See:
		// aerospike-client-c/src/test/aerospike_geo/query_geospatial.c:
		// predexp_points_within_region

		Expect(cnt).To(BeNumerically("==", 6))
	})

	It("predexp geo PIR query must work using Predicate API", func() {

		region :=
			"{ " +
				"    \"type\": \"Polygon\", " +
				"    \"coordinates\": [ " +
				"        [[-122.500000, 37.000000],[-121.000000, 37.000000], " +
				"         [-121.000000, 38.080000],[-122.500000, 38.080000], " +
				"         [-122.500000, 37.000000]] " +
				"    ] " +
				"}"

		stm := NewStatement(ns, set)
		stm.SetPredicate(BinValue("locval").GeoWithin(region))
		// stm.AddPredExp(NewPredExpGeoJSONValue(region))
		// stm.AddPredExp(NewPredExpGeoJSONBin("locval"))
		// stm.AddPredExp(NewPredExpGeoJSONWithin())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Correct answer is 6.  See:
		// aerospike-client-c/src/test/aerospike_geo/query_geospatial.c:
		// predexp_points_within_region

		Expect(cnt).To(BeNumerically("==", 6))
	})

	It("predexp geo RCP query must work", func() {

		point :=
			"{ " +
				"    \"type\": \"Point\", " +
				"    \"coordinates\": [ -122.0986857, 37.4214209 ] " +
				"}"

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpGeoJSONValue(point))
		stm.AddPredExp(NewPredExpGeoJSONBin("rgnval"))
		stm.AddPredExp(NewPredExpGeoJSONContains())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		// Correct answer is 6.  See:
		// aerospike-client-c/src/test/aerospike_geo/query_geospatial.c:
		// predexp_points_within_region

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Correct answer is 5.  See:
		// aerospike-client-c/src/test/aerospike_geo/query_geospatial.c:
		// predexp_regions_containing_point

		Expect(cnt).To(BeNumerically("==", 5))
	})

	It("predexp geo RCP query must work using Predicate API", func() {

		point :=
			"{ " +
				"    \"type\": \"Point\", " +
				"    \"coordinates\": [ -122.0986857, 37.4214209 ] " +
				"}"

		stm := NewStatement(ns, set)
		stm.SetPredicate(BinValue("rgnval").GeoContains(point))
		// stm.AddPredExp(NewPredExpGeoJSONValue(point))
		// stm.AddPredExp(NewPredExpGeoJSONBin("rgnval"))
		// stm.AddPredExp(NewPredExpGeoJSONContains())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		// Correct answer is 6.  See:
		// aerospike-client-c/src/test/aerospike_geo/query_geospatial.c:
		// predexp_points_within_region

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Correct answer is 5.  See:
		// aerospike-client-c/src/test/aerospike_geo/query_geospatial.c:
		// predexp_regions_containing_point

		Expect(cnt).To(BeNumerically("==", 5))
	})

	It("predexp last_update must work", func() {

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpIntegerValue(gaptime))
		stm.AddPredExp(NewPredExpLastUpdate())
		stm.AddPredExp(NewPredExpIntegerGreater())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// The answer should be 1000 - 333 = 667

		Expect(cnt).To(BeNumerically("==", 667))
	})

	It("predexp last_update must work using Predicate API", func() {

		stm := NewStatement(ns, set)
		stm.SetPredicate(RecLastUpdate().GreaterThan(gaptime))
		// stm.AddPredExp(NewPredExpIntegerValue(gaptime))
		// stm.AddPredExp(NewPredExpLastUpdate())
		// stm.AddPredExp(NewPredExpIntegerGreater())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// The answer should be 1000 - 333 = 667

		Expect(cnt).To(BeNumerically("==", 667))
	})

	It("predexp void_time must work", func() {

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpIntegerValue(0))
		stm.AddPredExp(NewPredExpVoidTime())
		stm.AddPredExp(NewPredExpIntegerEqual())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// The answer should be 1000 - 333 = 667

		Expect(cnt).To(BeNumerically("==", 667))
	})

	It("predexp void_time must work using Predicate API", func() {

		stm := NewStatement(ns, set)
		stm.SetPredicate(RecExpiration().Equal(0))
		// stm.AddPredExp(NewPredExpIntegerValue(0))
		// stm.AddPredExp(NewPredExpVoidTime())
		// stm.AddPredExp(NewPredExpIntegerEqual())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// The answer should be 1000 - 333 = 667

		Expect(cnt).To(BeNumerically("==", 667))
	})

	It("predexp rec_size work", func() {

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpIntegerValue(12 * 1024))
		stm.AddPredExp(NewPredExpRecSize())
		stm.AddPredExp(NewPredExpIntegerGreaterEq())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Answer should roughly be 1000 - (12/16 * 1000) ~= 250 + ovhd

		Expect(cnt).To(BeNumerically(">", 250))
		Expect(cnt).To(BeNumerically("<", 300))
	})

	It("predexp rec_size work using Predicate API", func() {

		stm := NewStatement(ns, set)
		stm.SetPredicate(RecSize().EqualOrGreaterThan(12 * 1024))
		// stm.AddPredExp(NewPredExpIntegerValue(12 * 1024))
		// stm.AddPredExp(NewPredExpRecSize())
		// stm.AddPredExp(NewPredExpIntegerGreaterEq())
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Answer should roughly be 1000 - (12/16 * 1000) ~= 250 + ovhd

		Expect(cnt).To(BeNumerically(">", 250))
		Expect(cnt).To(BeNumerically("<", 300))
	})

	It("predexp list_iter_or work", func() {

		stm := NewStatement(ns, set)
		stm.AddPredExp(NewPredExpIntegerValue(17))
		stm.AddPredExp(NewPredExpIntegerVar("ff"))
		stm.AddPredExp(NewPredExpIntegerEqual())
		stm.AddPredExp(NewPredExpListBin("lstval"))
		stm.AddPredExp(NewPredExpListIterateOr("ff"))
		recordset, err := client.Query(nil, stm)
		Expect(err).ToNot(HaveOccurred())

		cnt := 0
		for res := range recordset.Results() {
			Expect(res.Err).ToNot(HaveOccurred())
			cnt++
		}

		// Answer should be ceil(1000 / 17) = 59

		Expect(cnt).To(BeNumerically("==", 59))
	})

})
