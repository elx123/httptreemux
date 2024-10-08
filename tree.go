package httptreemux

import (
	"fmt"
	"strings"
)

type node struct {
	path string

	//这个属性是用来描述子节点的数量,简而言之也就是当前节点下,有多少个子节点,包含子节点下的子节点
	priority int

	// The list of static children to check.
	staticIndices []byte
	staticChild   []*node

	// If none of the above match, check the wildcard children
	wildcardChild *node

	// If none of the above match, then we use the catch-all, if applicable.
	catchAllChild *node

	// Data for the node is below.

	addSlash   bool
	isCatchAll bool
	// If true, the head handler was set implicitly, so let it also be set explicitly.
	implicitHead bool
	// If this node is the end of the URL, then call the handler, if applicable.
	leafHandler map[string]HandlerFunc

	// The names of the parameters to apply.
	leafWildcardNames []string
}

// 这里简单的看出来是降序排序
func (n *node) sortStaticChild(i int) {
	for i > 0 && n.staticChild[i].priority > n.staticChild[i-1].priority {
		n.staticChild[i], n.staticChild[i-1] = n.staticChild[i-1], n.staticChild[i]
		n.staticIndices[i], n.staticIndices[i-1] = n.staticIndices[i-1], n.staticIndices[i]
		i -= 1
	}
}

// prefix tree mux 在leaf 上存 handler
func (n *node) setHandler(verb string, handler HandlerFunc, implicitHead bool) {
	if n.leafHandler == nil {
		n.leafHandler = make(map[string]HandlerFunc)
	}
	_, ok := n.leafHandler[verb]
	if ok && (verb != "HEAD" || !n.implicitHead) {
		panic(fmt.Sprintf("%s already handles %s", n.path, verb))
	}
	n.leafHandler[verb] = handler

	if verb == "HEAD" {
		n.implicitHead = implicitHead
	}
}

// wildcards虽然这个变量在上一层中没有使用，但是在递归调用中使用了
// inStaticToken同样
// 目前看inStaticToken是为了单独给/一个节点
func (n *node) addPath(path string, wildcards []string, inStaticToken bool) *node {
	leaf := len(path) == 0
	if leaf {
		if wildcards != nil {
			// Make sure the current wildcards are the same as the old ones.
			// If not then we have an ambiguous path.
			if n.leafWildcardNames != nil {
				if len(n.leafWildcardNames) != len(wildcards) {
					// This should never happen.
					panic("Reached leaf node with differing wildcard array length. Please report this as a bug.")
				}

				for i := 0; i < len(wildcards); i++ {
					if n.leafWildcardNames[i] != wildcards[i] {
						panic(fmt.Sprintf("Wildcards %v are ambiguous with wildcards %v",
							n.leafWildcardNames, wildcards))
					}
				}
			} else {
				// No wildcards yet, so just add the existing set.
				n.leafWildcardNames = wildcards
			}
		}

		return n
	}

	c := path[0]
	nextSlash := strings.Index(path, "/")
	var thisToken string
	var tokenEnd int

	if c == '/' {
		// Done processing the previous token, so reset inStaticToken to false.
		thisToken = "/"
		tokenEnd = 1
	} else if nextSlash == -1 {
		thisToken = path
		tokenEnd = len(path)
	} else {
		thisToken = path[0:nextSlash]
		tokenEnd = nextSlash
	}
	remainingPath := path[tokenEnd:]
	//这个remainingPath会交给最后处理,依然是一个新的结节

	if c == '*' && !inStaticToken {
		// Token starts with a *, so it's a catch-all
		thisToken = thisToken[1:]
		if n.catchAllChild == nil {
			n.catchAllChild = &node{path: thisToken, isCatchAll: true}
		}

		if path[1:] != n.catchAllChild.path {
			panic(fmt.Sprintf("Catch-all name in %s doesn't match %s. You probably tried to define overlapping catchalls",
				path, n.catchAllChild.path))
		}

		if nextSlash != -1 {
			panic("/ after catch-all found in " + path)
		}

		if wildcards == nil {
			wildcards = []string{thisToken}
		} else {
			wildcards = append(wildcards, thisToken)
		}
		n.catchAllChild.leafWildcardNames = wildcards

		return n.catchAllChild
	} else if c == ':' && !inStaticToken {
		// Token starts with a :
		thisToken = thisToken[1:]

		if wildcards == nil {
			wildcards = []string{thisToken}
		} else {
			wildcards = append(wildcards, thisToken)
		}

		if n.wildcardChild == nil {
			n.wildcardChild = &node{path: "wildcard"}
		}

		return n.wildcardChild.addPath(remainingPath, wildcards, false)

	} else {
		// if strings.ContainsAny(thisToken, ":*") {
		// 	panic("* or : in middle of path component " + path)
		// }
		//这里逻辑是作者自己加的，也可以理解为作者二次开发
		unescaped := false
		if len(thisToken) >= 2 && !inStaticToken {
			if thisToken[0] == '\\' && (thisToken[1] == '*' || thisToken[1] == ':' || thisToken[1] == '\\') {
				// The token starts with a character escaped by a backslash. Drop the backslash.
				c = thisToken[1]
				thisToken = thisToken[1:]
				unescaped = true
			}
		}

		// Set inStaticToken to ensure that the rest of this token is not mistaken
		// for a wildcard if a prefix split occurs at a '*' or ':'.
		inStaticToken = (c != '/')

		// Do we have an existing node that starts with the same letter?
		for i, index := range n.staticIndices {
			if c == index {
				// Yes. Split it based on the common prefix of the existing
				// node and the new one.
				//这里prefix返回的是公共节点也就是commonPrefix
				child, prefixSplit := n.splitCommonPrefix(i, thisToken)

				child.priority++
				n.sortStaticChild(i)
				if unescaped {
					// Account for the removed backslash.
					prefixSplit++
				}
				//这里prefixSplit是重点,因为刨去CommonPrefix这个节点,还剩下path[prefixSplit:],依然做addpath逻辑
				return child.addPath(path[prefixSplit:], wildcards, inStaticToken)
			}
		}

		// No existing node starting with this letter, so create it.
		child := &node{path: thisToken}
		//从这里可以看出handler是存于leaf节点中
		if n.staticIndices == nil {
			n.staticIndices = []byte{c}
			n.staticChild = []*node{child}
		} else {
			n.staticIndices = append(n.staticIndices, c)
			n.staticChild = append(n.staticChild, child)
		}
		return child.addPath(remainingPath, wildcards, inStaticToken)
	}
}

func (n *node) splitCommonPrefix(existingNodeIndex int, path string) (*node, int) {
	childNode := n.staticChild[existingNodeIndex]
	//这里将比如abcd,abc这样的例子剩下的d 交给上一层的addpath处理,也就是最终还会将d转入这一层
	if strings.HasPrefix(path, childNode.path) {
		// No split needs to be done. Rather, the new path shares the entire
		// prefix with the existing node, so the new node is just a child of
		// the existing one. Or the new path is the same as the existing path,
		// which means that we just move on to the next token. Either way,
		// this return accomplishes that
		return childNode, len(childNode.path)
	}
	//比方说abcd,abd
	//之后将原有的节点添加到新节点之后，原有的path中的节点比如cd就返回
	var i int
	// Find the length of the common prefix of the child node and the new path.
	for i = range childNode.path {
		if i == len(path) {
			break
		}
		if path[i] != childNode.path[i] {
			break
		}
	}

	commonPrefix := path[0:i]
	childNode.path = childNode.path[i:]
	//原先的path 改为childNode.path[i:],比如apple改为le
	// Create a new intermediary node in the place of the existing node, with
	// the existing node as a child.
	newNode := &node{
		path:     commonPrefix,
		priority: childNode.priority,
		// Index is the first letter of the non-common part of the path.
		staticIndices: []byte{childNode.path[0]},
		staticChild:   []*node{childNode},
	}
	n.staticChild[existingNodeIndex] = newNode

	return newNode, i
}

// 这里leaf node可能也会存path,但是在search的时候到达leaf node的时候path会为空，这里的path指代search path
func (n *node) search(method, path string) (found *node, handler HandlerFunc, params []string) {
	// if test != nil {
	// 	test.Logf("Searching for %s in %s", path, n.dumpTree("", ""))
	// }
	pathLen := len(path)
	if pathLen == 0 {
		if len(n.leafHandler) == 0 {
			return nil, nil, nil
		} else {
			return n, n.leafHandler[method], nil
		}
	}

	// First see if this matches a static token.
	firstChar := path[0]
	for i, staticIndex := range n.staticIndices {
		//这里我们能理解所有静态路径都是存储于staticChild
		//查询子节点从首字母开始
		if staticIndex == firstChar {
			child := n.staticChild[i]
			childPathLen := len(child.path)
			//这里的逻辑是target path已经搜索到底了,当前的这个节点已经是leaf节点,直接break出循环
			if pathLen >= childPathLen && child.path == path[:childPathLen] {
				nextPath := path[childPathLen:]
				//这里首先匹配首字母,然后直接匹配path
				found, handler, params = child.search(method, nextPath)
			}
			break
		}
	}

	// If we found a node and it had a valid handler, then return here. Otherwise
	// let's remember that we found this one, but look for a better match.
	if handler != nil {
		return
	}

	if n.wildcardChild != nil {
		// Didn't find a static token, so check for a wildcard.
		nextSlash := strings.IndexByte(path, '/')
		if nextSlash < 0 {
			nextSlash = pathLen
		}

		thisToken := path[0:nextSlash]
		nextToken := path[nextSlash:]

		if len(thisToken) > 0 { // Don't match on empty tokens.
			wcNode, wcHandler, wcParams := n.wildcardChild.search(method, nextToken)
			if wcHandler != nil || (found == nil && wcNode != nil) {
				unescaped, err := unescape(thisToken)
				if err != nil {
					unescaped = thisToken
				}

				if wcParams == nil {
					wcParams = []string{unescaped}
				} else {
					wcParams = append(wcParams, unescaped)
				}

				if wcHandler != nil {
					return wcNode, wcHandler, wcParams
				}

				// Didn't actually find a handler here, so remember that we
				// found a node but also see if we can fall through to the
				// catchall.
				found = wcNode
				handler = wcHandler
				params = wcParams
			}
		}
	}

	catchAllChild := n.catchAllChild
	if catchAllChild != nil {
		// Hit the catchall, so just assign the whole remaining path if it
		// has a matching handler.
		handler = catchAllChild.leafHandler[method]
		// Found a handler, or we found a catchall node without a handler.
		// Either way, return it since there's nothing left to check after this.
		if handler != nil || found == nil {
			unescaped, err := unescape(path)
			if err != nil {
				unescaped = path
			}

			return catchAllChild, handler, []string{unescaped}
		}

	}

	return found, handler, params
}

func (n *node) dumpTree(prefix, nodeType string) string {
	line := fmt.Sprintf("%s %02d %s%s [%d] %v wildcards %v\n", prefix, n.priority, nodeType, n.path,
		len(n.staticChild), n.leafHandler, n.leafWildcardNames)
	prefix += "  "
	for _, node := range n.staticChild {
		line += node.dumpTree(prefix, "")
	}
	if n.wildcardChild != nil {
		line += n.wildcardChild.dumpTree(prefix, ":")
	}
	if n.catchAllChild != nil {
		line += n.catchAllChild.dumpTree(prefix, "*")
	}
	return line
}
