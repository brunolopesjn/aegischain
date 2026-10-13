package main

type AccessChecker interface {
	CheckAccess(patientID, providerID, studyUID string) (bool, error)
}