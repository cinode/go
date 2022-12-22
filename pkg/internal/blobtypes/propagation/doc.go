// propagation package contains implementation of blob types for public propagation.
//
// This package is intentionally separated from the private part to ensure
// and easily validate that a binary operating on the public dataset only
// does not even contain the code to create new blob dataset.
package propagation
