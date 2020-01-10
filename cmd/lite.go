/*
Copyright © 2020 Jack Zampolin <jack.zampolin@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// liteCmd represents the lite command
var liteCmd = &cobra.Command{
	Use:   "lite",
	Short: "Commands to manage lite clients created by this relayer",
}

// liteCmd represents the lite command
var liteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Commands to manage lite clients created by this relayer",
}

// liteCmd represents the lite command
var liteShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Commands to manage lite clients created by this relayer",
}

// liteCmd represents the lite command
var liteDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Commands to manage lite clients created by this relayer",
}

// liteCmd represents the lite command
var liteUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Commands to manage lite clients created by this relayer",
}

func init() {
	rootCmd.AddCommand(liteCmd)
	liteCmd.AddCommand(liteCreateCmd)
	liteCmd.AddCommand(liteShowCmd)
	liteCmd.AddCommand(liteDeleteCmd)
	liteCmd.AddCommand(liteUpdateCmd)
}
