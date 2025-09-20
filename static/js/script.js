document.addEventListener('DOMContentLoaded', function() {
    // 添加菜单项的点击事件
    const menuItems = document.querySelectorAll('.menu a');
    
    menuItems.forEach(item => {
        item.addEventListener('click', function(e) {
            // 首页链接不需要特殊处理
            if (this.getAttribute('href') === '/') {
                return;
            }
            
            // 其他链接在当前页面打开
            e.preventDefault();
            const url = this.getAttribute('href');
            window.location.href = url;
        });
    });

    // 卡片悬停效果增强
    const cards = document.querySelectorAll('.card');
    
    cards.forEach(card => {
        card.addEventListener('mouseenter', function() {
            this.style.transform = 'translateY(-15px) scale(1.03)';
        });
        
        card.addEventListener('mouseleave', function() {
            this.style.transform = 'translateY(0) scale(1)';
        });
    });
});