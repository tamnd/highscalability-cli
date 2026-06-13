package cli

func isNotFound(_ error) bool {
	// The highscalability package does not define a sentinel ErrNotFound;
	// HTTP 404s are returned as generic errors. Keep the hook for future use.
	return false
}

func mapFetchErr(err error) error {
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return codeError(exitNoData, err)
	}
	return codeError(exitError, err)
}
