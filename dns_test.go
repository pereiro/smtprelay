package main

import (
	"testing"
	"net"
)

func Test_getRoundElement(t *testing.T) {

	var mxList []*net.MX
	mxList = append(mxList,&net.MX{Host:"1.1.1.1",Pref:10})
	fact,err := getRoundElement(mxList,0)
	if err != nil {
		t.Errorf("error while test")
	}
	expect := net.MX{Host:"1.1.1.1",Pref:10}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	fact,err = getRoundElement(mxList,1)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	fact,err = getRoundElement(mxList,15)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	mxList = append(mxList,&net.MX{Host:"2.2.2.2",Pref:20})


	fact,err = getRoundElement(mxList,0)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	fact,err = getRoundElement(mxList,2)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	fact,err = getRoundElement(mxList,4)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	expect = net.MX{Host:"2.2.2.2",Pref:20}

	fact,err = getRoundElement(mxList,1)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	fact,err = getRoundElement(mxList,3)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

	fact,err = getRoundElement(mxList,15)
	if err != nil {
		t.Errorf("error while test")
	}

	if !(fact.Host==expect.Host) {
		t.Errorf("expect '%v', got - '%v'", expect.Host, fact.Host)
	}

//	mxList = append(mxList,&net.MX{Host:"2.2.2.2",Pref:20})
//	fact,err := getRoundElement(mxList,0)
//	if err != nil {
//		t.Errorf("error while test")
//	}
//	expect := &net.MX{Host:"1.1.1.1",Pref:10}
//
//	if !reflect.DeepEqual(fact, expect) {
//		t.Errorf("expect '%v', got - '%v'", expect, fact)
//	}


}