import SwiftUI

struct ChipWrap: SwiftUI.Layout {
    var spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: SwiftUI.Layout.Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? Measure.reading
        let rows = place(in: width, subviews: subviews)
        let height = rows.last.map { $0.y + $0.height } ?? 0
        return CGSize(width: width, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: SwiftUI.Layout.Subviews, cache: inout ()) {
        let rows = place(in: bounds.width, subviews: subviews)
        for item in rows {
            item.subview.place(
                at: CGPoint(x: bounds.minX + item.x, y: bounds.minY + item.y),
                proposal: ProposedViewSize(width: item.width, height: item.height)
            )
        }
    }

    private func place(in width: CGFloat, subviews: SwiftUI.Layout.Subviews) -> [Row] {
        var rows: [Row] = []
        var x: CGFloat = 0
        var y: CGFloat = 0
        var rowHeight: CGFloat = 0
        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if x > 0, x + size.width > width {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            rows.append(Row(subview: subview, x: x, y: y, width: size.width, height: size.height))
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
        }
        return rows
    }

    private struct Row {
        var subview: SwiftUI.LayoutSubview
        var x: CGFloat
        var y: CGFloat
        var width: CGFloat
        var height: CGFloat
    }
}
