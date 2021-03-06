package main

const license = "This program is free software: you can redistribute it and/or modify " +
	"it under the terms of the GNU General Public License as published by " +
	"the Free Software Foundation, either version 3 of the License, or " +
	"(at your option) any later version. " +
	"This program is distributed in the hope that it will be useful, " +
	"but WITHOUT ANY WARRANTY; without even the implied warranty of " +
	"MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the " +
	"GNU General Public License for more details. " +
	"You should have received a copy of the GNU General Public License " +
	"along with this program.  If not, see <http://www.gnu.org/licenses/>. "

const licenceexp = `\s*licen[c|s]e\s*`

const currencyexp = `(\$|£|€|(Fr\.)|(SFr\.)|(FS)|(BTC)) ?(((\d{1,3}[, ])(\d{3}[, ])*\d{3})|\d+)( ?[.,] ?(\d{1,2}))?`
const floatexp = `^(\+|-)?(((\d{1,3}[, ])(\d{3}[ ,])*\d{3})|\d+)( ?([\.,]) ?(\d{3}[, ])*\d+)?$`
